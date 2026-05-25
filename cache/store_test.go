package cache_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/godx-jp/godx-platform-framework/cache"
	cdriver "github.com/godx-jp/godx-platform-framework/cache/driver"
)

func newMemoryStore(t *testing.T) *cache.Store {
	t.Helper()
	drv, err := cdriver.New(context.Background(), cdriver.Spec{Name: cdriver.DriverMemory})
	if err != nil {
		t.Fatalf("driver: %v", err)
	}
	s := cache.NewStore("test", drv, cache.StoreConfig{Driver: cdriver.DriverMemory})
	t.Cleanup(func() { _ = s.Shutdown(context.Background()) })
	return s
}

func TestStore_PullReturnsAndDeletes(t *testing.T) {
	s := newMemoryStore(t)
	ctx := context.Background()
	_ = s.Put(ctx, "k", []byte("v"), 0)
	v, ok, err := s.Pull(ctx, "k")
	if err != nil || !ok || string(v) != "v" {
		t.Fatalf("pull first: v=%q ok=%v err=%v", v, ok, err)
	}
	if _, ok, _ = s.Pull(ctx, "k"); ok {
		t.Fatal("pull second: still present")
	}
}

func TestStore_RememberCachesAndReuses(t *testing.T) {
	s := newMemoryStore(t)
	ctx := context.Background()
	calls := 0
	fn := func(context.Context) ([]byte, error) {
		calls++
		return []byte("fresh"), nil
	}
	v, err := s.Remember(ctx, "k", time.Minute, fn)
	if err != nil || string(v) != "fresh" {
		t.Fatalf("remember 1: v=%q err=%v", v, err)
	}
	v, err = s.Remember(ctx, "k", time.Minute, fn)
	if err != nil || string(v) != "fresh" {
		t.Fatalf("remember 2: v=%q err=%v", v, err)
	}
	if calls != 1 {
		t.Fatalf("fn called %d times; want 1 (second call should hit cache)", calls)
	}
}

func TestStore_RememberPropagatesError(t *testing.T) {
	s := newMemoryStore(t)
	ctx := context.Background()
	want := errors.New("boom")
	_, err := s.Remember(ctx, "k", time.Minute, func(context.Context) ([]byte, error) {
		return nil, want
	})
	if !errors.Is(err, want) {
		t.Fatalf("remember: want boom, got %v", err)
	}
	if ok, _ := s.Has(ctx, "k"); ok {
		t.Fatal("Remember stored a value after fn returned error")
	}
}

type payload struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func TestStore_JSONRoundTrip(t *testing.T) {
	s := newMemoryStore(t)
	ctx := context.Background()

	in := payload{Name: "alice", Count: 7}
	if err := s.PutJSON(ctx, "u", in, time.Minute); err != nil {
		t.Fatalf("putjson: %v", err)
	}
	var out payload
	ok, err := s.GetJSON(ctx, "u", &out)
	if err != nil || !ok || out != in {
		t.Fatalf("getjson: out=%+v ok=%v err=%v", out, ok, err)
	}

	calls := 0
	var fresh payload
	err = s.RememberJSON(ctx, "v", time.Minute, &fresh, func(context.Context) (any, error) {
		calls++
		return payload{Name: "bob", Count: 2}, nil
	})
	if err != nil || fresh != (payload{Name: "bob", Count: 2}) || calls != 1 {
		t.Fatalf("rememberjson initial: fresh=%+v calls=%d err=%v", fresh, calls, err)
	}
	// second call hits cache
	var cached payload
	err = s.RememberJSON(ctx, "v", time.Minute, &cached, func(context.Context) (any, error) {
		calls++
		return payload{}, nil
	})
	if err != nil || cached.Name != "bob" || calls != 1 {
		t.Fatalf("rememberjson cached: cached=%+v calls=%d err=%v", cached, calls, err)
	}
}

func TestStore_MissingMirrorsHas(t *testing.T) {
	s := newMemoryStore(t)
	ctx := context.Background()
	missing, _ := s.Missing(ctx, "absent")
	if !missing {
		t.Fatal("missing on absent key should be true")
	}
	_ = s.Put(ctx, "absent", []byte("v"), 0)
	missing, _ = s.Missing(ctx, "absent")
	if missing {
		t.Fatal("missing after put should be false")
	}
}
