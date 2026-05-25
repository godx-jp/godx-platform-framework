package config

import (
	"context"
	"sync"
	"testing"

	cdriver "github.com/godx-jp/godx-platform-framework/config/driver"
	"github.com/godx-jp/godx-platform-framework/config/drivers/static"
)

func TestRepositoryNilDataSafe(t *testing.T) {
	r := NewRepository(nil)
	if r.Has("anything") {
		t.Fatalf("nil-data repo should report Has==false")
	}
	if v, ok := r.Get("x"); ok || v != nil {
		t.Fatalf("nil-data Get should miss")
	}
	r.Set("a", 1)
	if r.GetInt("a", 0) != 1 {
		t.Fatalf("Set on nil-data must initialise")
	}
}

func TestRepositoryEmptyKey(t *testing.T) {
	r := NewRepository(map[string]any{"a": 1})
	r.Set("", "x")
	r.Forget("")
	if _, ok := r.Get(""); ok {
		t.Fatalf("Get on empty key should miss")
	}
}

func TestRepositoryDotIntermediateNonMap(t *testing.T) {
	r := NewRepository(map[string]any{"a": 1})
	r.Set("a.b", "child")
	if r.GetString("a.b", "") != "child" {
		t.Fatalf("Set should overwrite non-map intermediate")
	}
}

func TestManagerLoadErrorPreservedOnReload(t *testing.T) {
	// A source that always errors.
	mgr := NewManager()
	defer func() { _ = mgr.Shutdown(context.Background()) }()
	must(t, mgr.AddSource(context.Background(), "ok", static.New(map[string]any{"k": "v"})))
	if err := mgr.AddSource(context.Background(), "bad", errSource{}); err == nil {
		t.Fatalf("AddSource should surface load error")
	}
	// "ok" should still be intact after the failed AddSource roll-back.
	srcs := mgr.Sources()
	if len(srcs) != 1 || srcs[0] != "ok" {
		t.Fatalf("rolled-back chain unexpected: %v", srcs)
	}
}

type errSource struct{}

func (errSource) Name() string                                      { return "err" }
func (errSource) Load(context.Context) (map[string]any, error)      { return nil, cdriver.ErrClosed }
func (errSource) Shutdown(context.Context) error                    { return nil }

func TestManagerConcurrentReload(t *testing.T) {
	mgr := NewManager()
	defer func() { _ = mgr.Shutdown(context.Background()) }()
	must(t, mgr.AddSource(context.Background(), "s", static.New(map[string]any{"k": "v"})))
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = mgr.Reload(context.Background())
		}()
	}
	wg.Wait()
}

func TestModuleStoreKeyStable(t *testing.T) {
	if StoreKey == "" {
		t.Fatalf("StoreKey must not be empty")
	}
}
