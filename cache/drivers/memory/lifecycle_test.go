package memory_test

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"testing"
	"time"

	cdriver "github.com/godx-jp/godx-platform-framework/cache/driver"
	_ "github.com/godx-jp/godx-platform-framework/cache/drivers/memory"
)

func mk(t *testing.T) cdriver.Driver {
	t.Helper()
	d, err := cdriver.New(context.Background(), cdriver.Spec{Name: cdriver.DriverMemory})
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	return d
}

func TestMemory_ShutdownIsIdempotent(t *testing.T) {
	d := mk(t)
	ctx := context.Background()
	if err := d.Shutdown(ctx); err != nil {
		t.Fatalf("1st shutdown: %v", err)
	}
	if err := d.Shutdown(ctx); err != nil {
		t.Fatalf("2nd shutdown: %v", err)
	}
	if err := d.Shutdown(ctx); err != nil {
		t.Fatalf("3rd shutdown: %v", err)
	}
}

func TestMemory_OperationsAfterShutdownReturnErrClosed(t *testing.T) {
	d := mk(t)
	ctx := context.Background()
	_ = d.Shutdown(ctx)

	// Every mutating operation must surface ErrClosed (was a nil-map
	// panic before v0.7.1).
	if _, _, err := d.Get(ctx, "k"); !errors.Is(err, cdriver.ErrClosed) {
		t.Fatalf("Get: want ErrClosed, got %v", err)
	}
	if err := d.Put(ctx, "k", []byte("v"), 0); !errors.Is(err, cdriver.ErrClosed) {
		t.Fatalf("Put: want ErrClosed, got %v", err)
	}
	if _, err := d.Add(ctx, "k", []byte("v"), 0); !errors.Is(err, cdriver.ErrClosed) {
		t.Fatalf("Add: want ErrClosed, got %v", err)
	}
	if err := d.Forget(ctx, "k"); !errors.Is(err, cdriver.ErrClosed) {
		t.Fatalf("Forget: want ErrClosed, got %v", err)
	}
	if _, err := d.Has(ctx, "k"); !errors.Is(err, cdriver.ErrClosed) {
		t.Fatalf("Has: want ErrClosed, got %v", err)
	}
	if err := d.Flush(ctx); !errors.Is(err, cdriver.ErrClosed) {
		t.Fatalf("Flush: want ErrClosed, got %v", err)
	}
	if _, err := d.Increment(ctx, "k", 1); !errors.Is(err, cdriver.ErrClosed) {
		t.Fatalf("Increment: want ErrClosed, got %v", err)
	}
	if _, err := d.Decrement(ctx, "k", 1); !errors.Is(err, cdriver.ErrClosed) {
		t.Fatalf("Decrement: want ErrClosed, got %v", err)
	}
}

func TestMemory_SweeperGoroutineExits(t *testing.T) {
	before := runtime.NumGoroutine()
	for i := 0; i < 50; i++ {
		d := mk(t)
		_ = d.Put(context.Background(), "k", []byte("v"), 0)
		_ = d.Shutdown(context.Background())
	}
	// Give the runtime a beat to retire workers (the sweeper goroutine
	// leaves immediately on `done`, but goroutine accounting can lag
	// briefly).
	time.Sleep(50 * time.Millisecond)
	after := runtime.NumGoroutine()
	if after > before+5 {
		t.Fatalf("goroutine leak suspected: before=%d after=%d (50 cycles)", before, after)
	}
}

func TestMemory_NegativeAndZeroTTL(t *testing.T) {
	d := mk(t)
	defer d.Shutdown(context.Background())
	ctx := context.Background()

	// TTL == 0  ⇒ forever
	_ = d.Put(ctx, "forever", []byte("v"), 0)
	time.Sleep(40 * time.Millisecond)
	if _, ok, _ := d.Get(ctx, "forever"); !ok {
		t.Fatal("TTL=0 should mean forever")
	}

	// TTL < 0 is undocumented but must not crash; entry is permitted to
	// be invisible immediately. Either behaviour is acceptable —
	// guard against panics.
	if err := d.Put(ctx, "negative", []byte("v"), -1*time.Second); err != nil {
		t.Fatalf("negative TTL panicked / errored: %v", err)
	}
}

func TestMemory_ConcurrentReadersDontStomp(t *testing.T) {
	d := mk(t)
	defer d.Shutdown(context.Background())
	ctx := context.Background()
	_ = d.Put(ctx, "k", []byte("hello"), 0)

	const readers = 64
	var wg sync.WaitGroup
	wg.Add(readers)
	for i := 0; i < readers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				v, _, _ := d.Get(ctx, "k")
				if string(v) != "hello" {
					t.Errorf("reader saw mutated value: %q", v)
					return
				}
				// Mutate the returned buffer — must not affect the cache.
				if len(v) > 0 {
					v[0] = 'X'
				}
			}
		}()
	}
	wg.Wait()
	if v, _, _ := d.Get(ctx, "k"); string(v) != "hello" {
		t.Fatalf("cache leaked internal buffer; final value=%q", v)
	}
}
