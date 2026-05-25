package lock

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestMemoryTryAcquire(t *testing.T) {
	m := NewMemory()
	release, ok, err := m.TryAcquire(context.Background(), "job")
	if err != nil || !ok || release == nil {
		t.Fatalf("first acquire failed ok=%v err=%v", ok, err)
	}
	_, ok2, _ := m.TryAcquire(context.Background(), "job")
	if ok2 {
		t.Fatalf("second acquire should fail")
	}
	if err := release(); err != nil {
		t.Fatalf("release: %v", err)
	}
	_, ok3, _ := m.TryAcquire(context.Background(), "job")
	if !ok3 {
		t.Fatalf("acquire after release should succeed")
	}
}

func TestCacheRequiresStore(t *testing.T) {
	if _, err := NewCache(CacheOptions{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCacheDistributedLock(t *testing.T) {
	mc := &memStore{data: map[string][]byte{}}
	c, err := NewCache(CacheOptions{Store: mc, Owner: "host-a", TTL: time.Minute})
	if err != nil {
		t.Fatalf("NewCache: %v", err)
	}
	release, ok, err := c.TryAcquire(context.Background(), "nightly")
	if err != nil || !ok {
		t.Fatalf("acquire: ok=%v err=%v", ok, err)
	}
	_, ok2, _ := c.TryAcquire(context.Background(), "nightly")
	if ok2 {
		t.Fatalf("second holder should be rejected")
	}
	if err := release(); err != nil {
		t.Fatalf("release: %v", err)
	}
}

// randomToken must produce a unique value per call so two replicas sharing
// the same configured Owner never share an effective lock value (Finding A).
func TestRandomTokenUnique(t *testing.T) {
	seen := map[string]struct{}{}
	for i := 0; i < 1000; i++ {
		tok, err := randomToken()
		if err != nil {
			t.Fatalf("randomToken: %v", err)
		}
		if tok == "" {
			t.Fatalf("randomToken returned empty")
		}
		if _, dup := seen[tok]; dup {
			t.Fatalf("randomToken collision at %d: %q", i, tok)
		}
		seen[tok] = struct{}{}
	}
}

// Two acquisitions of the same Cache (same configured Owner) must store
// distinct, owner-prefixed tokens — proving the per-acquisition token, not the
// shared "owner" constant, is what guards the lock (Finding A).
func TestCacheTokenIsPerAcquisitionAndOwnerScoped(t *testing.T) {
	mc := &renewableStore{data: map[string][]byte{}}
	c, err := NewCache(CacheOptions{Store: mc, Owner: "host-a", TTL: time.Minute})
	if err != nil {
		t.Fatalf("NewCache: %v", err)
	}

	release1, ok, err := c.TryAcquire(context.Background(), "job")
	if err != nil || !ok {
		t.Fatalf("first acquire: ok=%v err=%v", ok, err)
	}
	tok1 := append([]byte(nil), mc.get("schedule-lock:job")...)
	if !strings.HasPrefix(string(tok1), "host-a:") {
		t.Fatalf("token %q must be prefixed with configured owner", tok1)
	}
	if err := release1(); err != nil {
		t.Fatalf("release1: %v", err)
	}

	if _, ok, err := c.TryAcquire(context.Background(), "job"); err != nil || !ok {
		t.Fatalf("re-acquire: ok=%v err=%v", ok, err)
	}
	tok2 := mc.get("schedule-lock:job")
	if bytes.Equal(tok1, tok2) {
		t.Fatalf("two acquisitions produced identical tokens %q — per-acquisition token missing", tok1)
	}
}

// Compare-and-delete proof (Finding A): a Renew carrying a stale token must NOT
// affect a lock now held under a different token. renewableStore mirrors the
// Redis Lua semantics: Renew only succeeds when the stored value matches.
func TestRenewWithStaleTokenDoesNotStealLock(t *testing.T) {
	mc := &renewableStore{data: map[string][]byte{}}
	key := "schedule-lock:job"

	// Replica B currently owns the lock under token "host-b:bbb".
	bToken := []byte("host-b:bbb")
	if added, err := mc.Add(context.Background(), key, bToken, time.Minute); err != nil || !added {
		t.Fatalf("seed B lock: added=%v err=%v", added, err)
	}

	// Replica A's renew loop fires with its OLD token "host-a:aaa" (A's lock
	// expired and B took over). A value-checked Renew must reject it.
	if err := mc.Renew(context.Background(), key, []byte("host-a:aaa"), time.Minute); err == nil {
		t.Fatalf("stale-token renew must fail, but succeeded")
	}
	if got := mc.get(key); !bytes.Equal(got, bToken) {
		t.Fatalf("stale renew mutated B's lock: got %q want %q", got, bToken)
	}

	// B's own renew (matching token) must still succeed.
	if err := mc.Renew(context.Background(), key, bToken, time.Minute); err != nil {
		t.Fatalf("owner renew must succeed: %v", err)
	}
}

type memStore struct {
	data map[string][]byte
}

func (m *memStore) Add(_ context.Context, key string, value []byte, _ time.Duration) (bool, error) {
	if _, ok := m.data[key]; ok {
		return false, nil
	}
	m.data[key] = value
	return true, nil
}

func (m *memStore) Forget(_ context.Context, key string) error {
	delete(m.data, key)
	return nil
}

// renewableStore mirrors the Redis compare-and-set Lua semantics in memory:
// Add is set-if-absent, Renew is value-checked (fails when the stored value no
// longer matches), and Forget is value-blind (matching the CacheStore
// contract, which carries no token — see RedisStore.Forget limitation).
type renewableStore struct {
	mu   sync.Mutex
	data map[string][]byte
}

func (m *renewableStore) get(key string) []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]byte(nil), m.data[key]...)
}

func (m *renewableStore) Add(_ context.Context, key string, value []byte, _ time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.data[key]; ok {
		return false, nil
	}
	m.data[key] = append([]byte(nil), value...)
	return true, nil
}

func (m *renewableStore) Forget(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
	return nil
}

func (m *renewableStore) Renew(_ context.Context, key string, value []byte, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cur, ok := m.data[key]
	if !ok || !bytes.Equal(cur, value) {
		return errStaleRenew
	}
	m.data[key] = append([]byte(nil), value...)
	return nil
}

var errStaleRenew = staleErr("scheduler/lock: renew lost key")

type staleErr string

func (e staleErr) Error() string { return string(e) }
