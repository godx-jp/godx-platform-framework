//go:build integration

// Integration smoke test for the redis cache driver. Hits a real
// redis-server — does NOT run in normal `go test`.
//
// Boot redis in another shell:
//
//	docker run --rm -d --name redis-test -p 6379:6379 redis:7
//
// Then run:
//
//	go test -tags integration -run TestRedis_Integration -v ./cache/drivers/redis/
//
// Env override (optional):
//
//	REDIS_TEST_URL  default redis://127.0.0.1:6379/15
//	                (uses logical DB 15 to avoid clobbering other data)
package redis_test

import (
	"context"
	"errors"
	"net"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	cdriver "github.com/godx-jp/godx-platform-framework/cache/driver"
	_ "github.com/godx-jp/godx-platform-framework/cache/drivers/redis"
)

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func mustReachable(t *testing.T, addr string) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Skipf("integration: %s unreachable (%v); skipping. boot redis per file header.", addr, err)
	}
	_ = conn.Close()
}

func newDriver(t *testing.T) cdriver.Driver {
	t.Helper()
	url := envOr("REDIS_TEST_URL", "redis://127.0.0.1:6379/15")
	// Parse host:port out of the URL so we can skip cleanly when not running.
	host := "127.0.0.1:6379"
	if u, err := parseRedisHost(url); err == nil {
		host = u
	}
	mustReachable(t, host)
	d, err := cdriver.New(context.Background(), cdriver.Spec{
		Name:   cdriver.DriverRedis,
		URL:    url,
		Prefix: "godx-test:" + t.Name() + ":",
	})
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	t.Cleanup(func() {
		_ = d.Flush(context.Background())
		_ = d.Shutdown(context.Background())
	})
	return d
}

func parseRedisHost(redisURL string) (string, error) {
	// redis://[user:pass@]host:port[/db]
	s := strings.TrimPrefix(redisURL, "redis://")
	s = strings.TrimPrefix(s, "rediss://")
	if at := strings.LastIndex(s, "@"); at >= 0 {
		s = s[at+1:]
	}
	if slash := strings.Index(s, "/"); slash >= 0 {
		s = s[:slash]
	}
	if s == "" {
		return "", errors.New("empty host")
	}
	return s, nil
}

func TestRedis_Integration_PutGetTTLForget(t *testing.T) {
	d := newDriver(t)
	ctx := context.Background()

	if err := d.Put(ctx, "k", []byte("v"), 0); err != nil {
		t.Fatalf("put: %v", err)
	}
	v, ok, err := d.Get(ctx, "k")
	if err != nil || !ok || string(v) != "v" {
		t.Fatalf("get: v=%q ok=%v err=%v", v, ok, err)
	}

	// TTL
	if err := d.Put(ctx, "ttl-key", []byte("expires"), 30*time.Millisecond); err != nil {
		t.Fatalf("put ttl: %v", err)
	}
	time.Sleep(60 * time.Millisecond)
	if _, ok, _ := d.Get(ctx, "ttl-key"); ok {
		t.Fatal("ttl-key should have expired")
	}

	_ = d.Forget(ctx, "k")
	if _, ok, _ := d.Get(ctx, "k"); ok {
		t.Fatal("forget did not remove key")
	}
}

func TestRedis_Integration_AddAndCounters(t *testing.T) {
	d := newDriver(t)
	ctx := context.Background()

	ok, _ := d.Add(ctx, "guard", []byte("first"), 0)
	if !ok {
		t.Fatal("first add: want true")
	}
	ok, _ = d.Add(ctx, "guard", []byte("second"), 0)
	if ok {
		t.Fatal("second add on existing key: want false")
	}

	const goroutines = 50
	const each = 10
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < each; j++ {
				if _, err := d.Increment(ctx, "counter", 1); err != nil {
					t.Errorf("incr: %v", err)
				}
			}
		}()
	}
	wg.Wait()

	v, _, err := d.Get(ctx, "counter")
	if err != nil {
		t.Fatalf("get counter: %v", err)
	}
	if string(v) != "500" {
		t.Fatalf("counter = %q, want 500", v)
	}
}

func TestRedis_Integration_FlushScopedByPrefix(t *testing.T) {
	d := newDriver(t)
	ctx := context.Background()
	_ = d.Put(ctx, "x1", []byte("1"), 0)
	_ = d.Put(ctx, "x2", []byte("2"), 0)
	if err := d.Flush(ctx); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if _, ok, _ := d.Get(ctx, "x1"); ok {
		t.Fatal("flush did not remove prefixed key")
	}
}
