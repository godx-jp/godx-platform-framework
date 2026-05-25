package cache_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	cdriver "github.com/godx-jp/godx-platform-framework/cache/driver"
	_ "github.com/godx-jp/godx-platform-framework/cache/drivers/file"
	_ "github.com/godx-jp/godx-platform-framework/cache/drivers/memory"
	_ "github.com/godx-jp/godx-platform-framework/cache/drivers/redis"
)

// Conformance is the cross-driver behaviour matrix. Every shipped
// cache driver must pass every scenario in this file — that's the
// guarantee `cache.Store` builds on. A new driver added to the
// framework should land alongside an entry in driverFactories below
// and a green run of `go test -race -run TestConformance ./cache`.
//
// Most scenarios are pure in-process tests. The "redis" backend boots
// only when a redis-server is reachable on the default port (and the
// REDIS_TEST_URL override accepted by the integration suite).
// Otherwise the redis row is skipped with t.Skip — never a failure.

type driverFactory struct {
	name string
	// build returns a driver wired with the supplied prefix. The
	// caller's t.Cleanup is responsible for Shutdown.
	build func(t *testing.T, prefix string) cdriver.Driver
	// note explains any per-driver quirks the matrix has to tolerate.
	note string
	// honoursContext is true when the driver actually pays attention
	// to ctx cancellation (redis does; in-process drivers don't —
	// they're synchronous and finish before the cancel fires).
	honoursContext bool
	// flushScopedByPrefix is true when Flush only removes keys with
	// the configured prefix (memory + redis). The file driver hashes
	// keys, so its Flush wipes the entire root.
	flushScopedByPrefix bool
}

func driverFactories(t *testing.T) []driverFactory {
	t.Helper()
	facs := []driverFactory{
		{
			name: "memory",
			build: func(t *testing.T, prefix string) cdriver.Driver {
				d, err := cdriver.New(context.Background(), cdriver.Spec{
					Name:   cdriver.DriverMemory,
					Prefix: prefix,
				})
				if err != nil {
					t.Fatalf("memory: %v", err)
				}
				t.Cleanup(func() { _ = d.Shutdown(context.Background()) })
				return d
			},
			flushScopedByPrefix: true,
		},
		{
			name: "file",
			build: func(t *testing.T, prefix string) cdriver.Driver {
				d, err := cdriver.New(context.Background(), cdriver.Spec{
					Name:   cdriver.DriverFile,
					Path:   t.TempDir(),
					Prefix: prefix,
				})
				if err != nil {
					t.Fatalf("file: %v", err)
				}
				t.Cleanup(func() { _ = d.Shutdown(context.Background()) })
				return d
			},
			note:                "Flush wipes root (filenames are hashed); prefix isolation needs separate Path",
			flushScopedByPrefix: false,
		},
	}
	if redisReachable() {
		facs = append(facs, driverFactory{
			name: "redis",
			build: func(t *testing.T, prefix string) cdriver.Driver {
				d, err := cdriver.New(context.Background(), cdriver.Spec{
					Name:   cdriver.DriverRedis,
					URL:    redisTestURL(),
					Prefix: prefix,
				})
				if err != nil {
					t.Fatalf("redis: %v", err)
				}
				t.Cleanup(func() {
					_ = d.Flush(context.Background())
					_ = d.Shutdown(context.Background())
				})
				return d
			},
			honoursContext:      true,
			flushScopedByPrefix: true,
		})
	} else {
		t.Logf("redis not reachable at %s — redis row skipped from conformance matrix", defaultRedisAddr())
	}
	return facs
}

func defaultRedisAddr() string {
	if v := os.Getenv("REDIS_TEST_URL"); v != "" {
		s := strings.TrimPrefix(v, "redis://")
		s = strings.TrimPrefix(s, "rediss://")
		if at := strings.LastIndex(s, "@"); at >= 0 {
			s = s[at+1:]
		}
		if slash := strings.Index(s, "/"); slash >= 0 {
			s = s[:slash]
		}
		if s != "" {
			return s
		}
	}
	return "127.0.0.1:6379"
}

func redisTestURL() string {
	if v := os.Getenv("REDIS_TEST_URL"); v != "" {
		return v
	}
	return "redis://127.0.0.1:6379/15"
}

func redisReachable() bool {
	c, err := net.DialTimeout("tcp", defaultRedisAddr(), 200*time.Millisecond)
	if err != nil {
		return false
	}
	_ = c.Close()
	return true
}

// ────────────────────────────────────────────────────────────────────
//                            scenarios
// ────────────────────────────────────────────────────────────────────

func TestConformance_RoundTrip(t *testing.T) {
	t.Parallel()
	forEachDriver(t, "rt:", func(t *testing.T, fac driverFactory, d cdriver.Driver) {
		ctx := context.Background()

		// Empty existence
		if ok, err := d.Has(ctx, "missing"); err != nil || ok {
			t.Fatalf("has-missing: ok=%v err=%v", ok, err)
		}
		if _, ok, err := d.Get(ctx, "missing"); err != nil || ok {
			t.Fatalf("get-missing: ok=%v err=%v", ok, err)
		}

		// Round trip
		if err := d.Put(ctx, "k", []byte("v"), 0); err != nil {
			t.Fatalf("put: %v", err)
		}
		if ok, err := d.Has(ctx, "k"); err != nil || !ok {
			t.Fatalf("has-present: ok=%v err=%v", ok, err)
		}
		if v, ok, err := d.Get(ctx, "k"); err != nil || !ok || string(v) != "v" {
			t.Fatalf("get-present: v=%q ok=%v err=%v", v, ok, err)
		}

		// Overwrite
		if err := d.Put(ctx, "k", []byte("v2"), 0); err != nil {
			t.Fatalf("overwrite: %v", err)
		}
		if v, _, _ := d.Get(ctx, "k"); string(v) != "v2" {
			t.Fatalf("overwrite read: v=%q", v)
		}

		// Forget removes; second forget is a no-op
		if err := d.Forget(ctx, "k"); err != nil {
			t.Fatalf("forget: %v", err)
		}
		if err := d.Forget(ctx, "k"); err != nil {
			t.Fatalf("forget-missing: %v", err)
		}
		if ok, _ := d.Has(ctx, "k"); ok {
			t.Fatal("has after forget: still present")
		}
	})
}

func TestConformance_TTL(t *testing.T) {
	t.Parallel()
	forEachDriver(t, "ttl:", func(t *testing.T, fac driverFactory, d cdriver.Driver) {
		ctx := context.Background()

		// TTL > 0 expires
		if err := d.Put(ctx, "expires", []byte("v"), 40*time.Millisecond); err != nil {
			t.Fatalf("put ttl: %v", err)
		}
		if _, ok, _ := d.Get(ctx, "expires"); !ok {
			t.Fatal("expires: should be visible immediately after Put")
		}
		time.Sleep(120 * time.Millisecond)
		if _, ok, _ := d.Get(ctx, "expires"); ok {
			t.Fatal("expires: still visible after TTL elapsed")
		}

		// TTL == 0 stores forever
		if err := d.Put(ctx, "forever", []byte("v"), 0); err != nil {
			t.Fatalf("put forever: %v", err)
		}
		time.Sleep(60 * time.Millisecond)
		if _, ok, _ := d.Get(ctx, "forever"); !ok {
			t.Fatal("forever: should still be visible")
		}
	})
}

func TestConformance_Add(t *testing.T) {
	t.Parallel()
	forEachDriver(t, "add:", func(t *testing.T, fac driverFactory, d cdriver.Driver) {
		ctx := context.Background()

		ok, err := d.Add(ctx, "k", []byte("first"), 0)
		if err != nil || !ok {
			t.Fatalf("add 1: ok=%v err=%v", ok, err)
		}
		ok, err = d.Add(ctx, "k", []byte("second"), 0)
		if err != nil || ok {
			t.Fatalf("add 2 (collision): ok=%v err=%v", ok, err)
		}
		if v, _, _ := d.Get(ctx, "k"); string(v) != "first" {
			t.Fatalf("value mutated by losing Add: %q", v)
		}

		// Add to expired key should succeed
		_ = d.Put(ctx, "exp", []byte("old"), 30*time.Millisecond)
		time.Sleep(80 * time.Millisecond)
		ok, err = d.Add(ctx, "exp", []byte("fresh"), 0)
		if err != nil || !ok {
			t.Fatalf("add to expired: ok=%v err=%v", ok, err)
		}
		if v, _, _ := d.Get(ctx, "exp"); string(v) != "fresh" {
			t.Fatalf("expired add value: %q", v)
		}
	})
}

func TestConformance_Counters(t *testing.T) {
	t.Parallel()
	forEachDriver(t, "ctr:", func(t *testing.T, fac driverFactory, d cdriver.Driver) {
		ctx := context.Background()

		// Increment on absent key starts at 0
		n, err := d.Increment(ctx, "c", 5)
		if err != nil || n != 5 {
			t.Fatalf("incr absent: n=%d err=%v", n, err)
		}
		n, err = d.Increment(ctx, "c", 3)
		if err != nil || n != 8 {
			t.Fatalf("incr existing: n=%d err=%v", n, err)
		}
		n, err = d.Decrement(ctx, "c", 2)
		if err != nil || n != 6 {
			t.Fatalf("decr: n=%d err=%v", n, err)
		}

		// Negative delta on Increment, positive delta on Decrement
		// must round-trip cleanly.
		_, _ = d.Increment(ctx, "c", -1) // 5
		n, _ = d.Decrement(ctx, "c", -2) // 5 - (-2) = 7
		if n != 7 {
			t.Fatalf("decr neg delta: n=%d", n)
		}

		// Non-integer rejection
		_ = d.Put(ctx, "bad", []byte("xyz"), 0)
		if _, err := d.Increment(ctx, "bad", 1); !errors.Is(err, cdriver.ErrNotInteger) {
			t.Fatalf("incr on non-int: want ErrNotInteger, got %v", err)
		}
		if _, err := d.Decrement(ctx, "bad", 1); !errors.Is(err, cdriver.ErrNotInteger) {
			t.Fatalf("decr on non-int: want ErrNotInteger, got %v", err)
		}

		// Counter survives Forget (next increment starts at 0 again).
		_ = d.Forget(ctx, "c")
		n, err = d.Increment(ctx, "c", 100)
		if err != nil || n != 100 {
			t.Fatalf("incr after forget: n=%d err=%v", n, err)
		}
	})
}

func TestConformance_ConcurrentCounters(t *testing.T) {
	t.Parallel()
	forEachDriver(t, "race:", func(t *testing.T, fac driverFactory, d cdriver.Driver) {
		ctx := context.Background()
		const goroutines = 32
		const perGoroutine = 25
		var wg sync.WaitGroup
		wg.Add(goroutines)
		for i := 0; i < goroutines; i++ {
			go func() {
				defer wg.Done()
				for j := 0; j < perGoroutine; j++ {
					if _, err := d.Increment(ctx, "counter", 1); err != nil {
						t.Errorf("incr: %v", err)
						return
					}
				}
			}()
		}
		wg.Wait()
		v, ok, err := d.Get(ctx, "counter")
		if err != nil || !ok {
			t.Fatalf("get counter: ok=%v err=%v", ok, err)
		}
		want := int64(goroutines * perGoroutine)
		got, _ := strconv.ParseInt(string(v), 10, 64)
		if got != want {
			t.Fatalf("counter = %d, want %d (lost updates — atomicity broken)", got, want)
		}
	})
}

func TestConformance_ConcurrentAdd(t *testing.T) {
	t.Parallel()
	forEachDriver(t, "race:", func(t *testing.T, fac driverFactory, d cdriver.Driver) {
		ctx := context.Background()
		const goroutines = 32
		var wg sync.WaitGroup
		wg.Add(goroutines)
		wins := make(chan bool, goroutines)
		for i := 0; i < goroutines; i++ {
			go func() {
				defer wg.Done()
				ok, _ := d.Add(ctx, "lockout", []byte("v"), 0)
				wins <- ok
			}()
		}
		wg.Wait()
		close(wins)
		count := 0
		for v := range wins {
			if v {
				count++
			}
		}
		if count != 1 {
			t.Fatalf("Add must succeed exactly once under contention; got %d", count)
		}
	})
}

func TestConformance_Pull(t *testing.T) {
	t.Parallel()
	forEachDriver(t, "pull:", func(t *testing.T, fac driverFactory, d cdriver.Driver) {
		ctx := context.Background()
		_ = d.Put(ctx, "flash", []byte("once"), 0)
		// Inline the Store-level Pull behaviour against the raw driver.
		v, ok, err := d.Get(ctx, "flash")
		if err != nil || !ok || string(v) != "once" {
			t.Fatalf("get: v=%q ok=%v err=%v", v, ok, err)
		}
		if err := d.Forget(ctx, "flash"); err != nil {
			t.Fatalf("forget: %v", err)
		}
		if _, ok, _ := d.Get(ctx, "flash"); ok {
			t.Fatal("pull effect: still present")
		}
	})
}

func TestConformance_FlushScope(t *testing.T) {
	t.Parallel()
	forEachDriver(t, "flushA:", func(t *testing.T, fac driverFactory, d cdriver.Driver) {
		ctx := context.Background()
		for _, k := range []string{"a", "b", "c"} {
			_ = d.Put(ctx, k, []byte(k), 0)
		}
		if err := d.Flush(ctx); err != nil {
			t.Fatalf("flush: %v", err)
		}
		for _, k := range []string{"a", "b", "c"} {
			if ok, _ := d.Has(ctx, k); ok {
				t.Fatalf("flush left %q behind", k)
			}
		}
	})
}

func TestConformance_FlushIsolation(t *testing.T) {
	// Two drivers on the same backend but with different prefixes —
	// Flush from one must not blow away keys from the other (for
	// drivers that scope Flush by prefix; file driver opt-out).
	t.Parallel()
	for _, fac := range driverFactories(t) {
		fac := fac
		if !fac.flushScopedByPrefix {
			t.Logf("[%s] skipped: %s", fac.name, fac.note)
			continue
		}
		t.Run(fac.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			a := fac.build(t, fmt.Sprintf("iso-A-%d:", time.Now().UnixNano()))
			b := fac.build(t, fmt.Sprintf("iso-B-%d:", time.Now().UnixNano()))
			if err := a.Put(ctx, "x", []byte("A"), 0); err != nil {
				t.Fatalf("a.Put: %v", err)
			}
			if err := b.Put(ctx, "x", []byte("B"), 0); err != nil {
				t.Fatalf("b.Put: %v", err)
			}
			if err := a.Flush(ctx); err != nil {
				t.Fatalf("a.Flush: %v", err)
			}
			if _, ok, _ := b.Get(ctx, "x"); !ok {
				t.Fatal("prefix isolation broken: b's key was deleted by a.Flush")
			}
		})
	}
}

func TestConformance_BoundaryValues(t *testing.T) {
	t.Parallel()
	forEachDriver(t, "bd:", func(t *testing.T, fac driverFactory, d cdriver.Driver) {
		ctx := context.Background()

		// Empty value round trip
		if err := d.Put(ctx, "empty", []byte{}, 0); err != nil {
			t.Fatalf("put empty: %v", err)
		}
		v, ok, err := d.Get(ctx, "empty")
		if err != nil || !ok {
			t.Fatalf("get empty: ok=%v err=%v", ok, err)
		}
		if len(v) != 0 {
			t.Fatalf("empty value len=%d (want 0); v=%v", len(v), v)
		}

		// Binary value with NUL and high-bit bytes
		binary := []byte{0x00, 0x01, 0x7f, 0x80, 0xff, 0x00, 0xab}
		if err := d.Put(ctx, "bin", binary, 0); err != nil {
			t.Fatalf("put bin: %v", err)
		}
		got, _, _ := d.Get(ctx, "bin")
		if string(got) != string(binary) {
			t.Fatalf("binary round trip mismatch: %v vs %v", got, binary)
		}

		// 1 MiB value
		big := make([]byte, 1<<20)
		for i := range big {
			big[i] = byte(i)
		}
		if err := d.Put(ctx, "big", big, 0); err != nil {
			t.Fatalf("put big: %v", err)
		}
		got, _, _ = d.Get(ctx, "big")
		if len(got) != len(big) {
			t.Fatalf("big len = %d, want %d", len(got), len(big))
		}
		if string(got[:64]) != string(big[:64]) || string(got[len(got)-64:]) != string(big[len(big)-64:]) {
			t.Fatal("big payload mismatch at boundaries")
		}

		// UTF-8 key
		key := "ユーザー:ベトナム/東京?id=1&x=2"
		if err := d.Put(ctx, key, []byte("ok"), 0); err != nil {
			t.Fatalf("put utf8 key: %v", err)
		}
		if v, ok, _ := d.Get(ctx, key); !ok || string(v) != "ok" {
			t.Fatalf("get utf8 key: v=%q ok=%v", v, ok)
		}

		// Long key (~512 bytes)
		long := strings.Repeat("a", 512)
		if err := d.Put(ctx, long, []byte("L"), 0); err != nil {
			t.Fatalf("put long key: %v", err)
		}
		if v, ok, _ := d.Get(ctx, long); !ok || string(v) != "L" {
			t.Fatalf("get long key: v=%q ok=%v", v, ok)
		}
	})
}

func TestConformance_ContextCancellation(t *testing.T) {
	t.Parallel()
	for _, fac := range driverFactories(t) {
		fac := fac
		if !fac.honoursContext {
			// Skip in-process drivers — they're synchronous and would
			// always finish before the cancellation fires.
			continue
		}
		t.Run(fac.name, func(t *testing.T) {
			t.Parallel()
			d := fac.build(t, "ctx:")
			ctx, cancel := context.WithCancel(context.Background())
			cancel() // already-cancelled
			if _, _, err := d.Get(ctx, "k"); err == nil {
				t.Fatal("expected error on already-cancelled ctx")
			}
		})
	}
}

// ────────────────────────────────────────────────────────────────────
//                            helpers
// ────────────────────────────────────────────────────────────────────

func forEachDriver(t *testing.T, prefix string, fn func(t *testing.T, fac driverFactory, d cdriver.Driver)) {
	t.Helper()
	facs := driverFactories(t)
	for _, fac := range facs {
		fac := fac
		t.Run(fac.name, func(t *testing.T) {
			t.Parallel()
			// Suffix the prefix with a per-test nanosecond stamp so
			// shared backends (redis) cannot collide between parallel
			// subtests.
			p := fmt.Sprintf("%s%d:%s:", prefix, time.Now().UnixNano(), t.Name())
			d := fac.build(t, p)
			fn(t, fac, d)
		})
	}
}
