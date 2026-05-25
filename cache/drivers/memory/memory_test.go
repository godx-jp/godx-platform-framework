package memory_test

import (
	"context"
	"errors"
	"testing"
	"time"

	cdriver "github.com/godx-jp/godx-platform-framework/cache/driver"
	_ "github.com/godx-jp/godx-platform-framework/cache/drivers/memory"
)

func newDriver(t *testing.T) cdriver.Driver {
	t.Helper()
	d, err := cdriver.New(context.Background(), cdriver.Spec{Name: cdriver.DriverMemory})
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	t.Cleanup(func() { _ = d.Shutdown(context.Background()) })
	return d
}

func TestMemory_PutGetForget(t *testing.T) {
	d := newDriver(t)
	ctx := context.Background()

	if err := d.Put(ctx, "k", []byte("v"), 0); err != nil {
		t.Fatalf("put: %v", err)
	}
	v, ok, err := d.Get(ctx, "k")
	if err != nil || !ok || string(v) != "v" {
		t.Fatalf("get: v=%q ok=%v err=%v", v, ok, err)
	}
	if err := d.Forget(ctx, "k"); err != nil {
		t.Fatalf("forget: %v", err)
	}
	if _, ok, _ := d.Get(ctx, "k"); ok {
		t.Fatal("get after forget: still present")
	}
}

func TestMemory_TTL_ExpiresOnRead(t *testing.T) {
	d := newDriver(t)
	ctx := context.Background()
	if err := d.Put(ctx, "k", []byte("v"), 10*time.Millisecond); err != nil {
		t.Fatalf("put: %v", err)
	}
	time.Sleep(25 * time.Millisecond)
	if _, ok, _ := d.Get(ctx, "k"); ok {
		t.Fatal("expected expired key to be invisible")
	}
}

func TestMemory_Add(t *testing.T) {
	d := newDriver(t)
	ctx := context.Background()
	ok, _ := d.Add(ctx, "k", []byte("first"), 0)
	if !ok {
		t.Fatal("first add: want true")
	}
	ok, _ = d.Add(ctx, "k", []byte("second"), 0)
	if ok {
		t.Fatal("second add on existing key: want false")
	}
	v, _, _ := d.Get(ctx, "k")
	if string(v) != "first" {
		t.Fatalf("value mutated: %q", v)
	}
}

func TestMemory_IncrementDecrement(t *testing.T) {
	d := newDriver(t)
	ctx := context.Background()
	n, err := d.Increment(ctx, "counter", 3)
	if err != nil || n != 3 {
		t.Fatalf("incr 3: n=%d err=%v", n, err)
	}
	n, err = d.Increment(ctx, "counter", 4)
	if err != nil || n != 7 {
		t.Fatalf("incr 4: n=%d err=%v", n, err)
	}
	n, err = d.Decrement(ctx, "counter", 2)
	if err != nil || n != 5 {
		t.Fatalf("decr 2: n=%d err=%v", n, err)
	}

	// Mismatched type -> ErrNotInteger
	_ = d.Put(ctx, "bad", []byte("hi"), 0)
	if _, err := d.Increment(ctx, "bad", 1); !errors.Is(err, cdriver.ErrNotInteger) {
		t.Fatalf("incr on non-int: want ErrNotInteger, got %v", err)
	}
}

func TestMemory_FlushScopedByPrefix(t *testing.T) {
	d, err := cdriver.New(context.Background(), cdriver.Spec{Name: cdriver.DriverMemory, Prefix: "app:"})
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	t.Cleanup(func() { _ = d.Shutdown(context.Background()) })
	ctx := context.Background()
	_ = d.Put(ctx, "a", []byte("1"), 0)
	_ = d.Put(ctx, "b", []byte("2"), 0)
	if err := d.Flush(ctx); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if _, ok, _ := d.Get(ctx, "a"); ok {
		t.Fatal("flush should have removed prefixed keys")
	}
}

func TestMemory_HasAndIsolation(t *testing.T) {
	d := newDriver(t)
	ctx := context.Background()
	if ok, _ := d.Has(ctx, "missing"); ok {
		t.Fatal("has missing: want false")
	}
	_ = d.Put(ctx, "present", []byte("v"), 0)
	if ok, _ := d.Has(ctx, "present"); !ok {
		t.Fatal("has present: want true")
	}
	// Mutating the returned slice must not affect the cache.
	v, _, _ := d.Get(ctx, "present")
	v[0] = 'x'
	v2, _, _ := d.Get(ctx, "present")
	if string(v2) != "v" {
		t.Fatalf("cache leaked internal buffer: %q", v2)
	}
}
