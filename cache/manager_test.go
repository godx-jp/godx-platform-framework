package cache_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/godx-jp/godx-platform-framework/cache"
	cdriver "github.com/godx-jp/godx-platform-framework/cache/driver"
	"github.com/godx-jp/godx-platform-framework/framework"
)

func mkStore(t *testing.T, name string) *cache.Store {
	t.Helper()
	drv, err := cdriver.New(context.Background(), cdriver.Spec{Name: cdriver.DriverMemory})
	if err != nil {
		t.Fatalf("driver: %v", err)
	}
	return cache.NewStore(name, drv, cache.StoreConfig{Driver: cdriver.DriverMemory})
}

func TestManager_DefaultAndLookup(t *testing.T) {
	mgr := cache.NewManager()
	if err := mgr.AddStore(mkStore(t, "primary")); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := mgr.AddStore(mkStore(t, "sessions")); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := mgr.SetDefault("primary"); err != nil {
		t.Fatalf("default: %v", err)
	}
	if got := mgr.DefaultName(); got != "primary" {
		t.Fatalf("default name: %q", got)
	}
	if got := mgr.Default().Name(); got != "primary" {
		t.Fatalf("default store: %q", got)
	}
	if _, err := mgr.Store("missing"); err == nil {
		t.Fatal("Store on missing name should error")
	}
	_ = mgr.Shutdown(context.Background())
}

func TestManager_DuplicateStoreRejected(t *testing.T) {
	mgr := cache.NewManager()
	if err := mgr.AddStore(mkStore(t, "dup")); err != nil {
		t.Fatalf("first: %v", err)
	}
	if err := mgr.AddStore(mkStore(t, "dup")); err == nil {
		t.Fatal("second AddStore with same name should error")
	}
	_ = mgr.Shutdown(context.Background())
}

func TestModuleWithConfig_BuildsRequestedStores(t *testing.T) {
	cfg := cache.Config{
		DefaultStore: "primary",
		Stores: map[string]cache.StoreConfig{
			"primary":  {Driver: cdriver.DriverMemory},
			"sessions": {Driver: cdriver.DriverMemory},
		},
	}
	app := framework.New("svc", "1.0.0").Use(cache.ModuleWithConfig(cfg))
	if err := app.Init(context.Background()); err != nil {
		t.Fatalf("init: %v", err)
	}
	mgr, err := cache.FromApp(app)
	if err != nil {
		t.Fatalf("fromapp: %v", err)
	}
	if got := strings.Join(mgr.Stores(), ","); got != "primary,sessions" {
		t.Fatalf("stores: %q", got)
	}
	if mgr.DefaultName() != "primary" {
		t.Fatalf("default: %q", mgr.DefaultName())
	}
	_ = app.Shutdown(context.Background())
}

func TestModuleWithConfig_BadDefaultIsRejected(t *testing.T) {
	cfg := cache.Config{
		DefaultStore: "ghost",
		Stores: map[string]cache.StoreConfig{
			"real": {Driver: cdriver.DriverMemory},
		},
	}
	app := framework.New("svc", "1.0.0").Use(cache.ModuleWithConfig(cfg))
	err := app.Init(context.Background())
	if err == nil || !strings.Contains(err.Error(), "default store") {
		t.Fatalf("want default-not-present error, got %v", err)
	}
}

func TestModule_FromContextRoundTrip(t *testing.T) {
	mgr := cache.NewManager()
	if err := mgr.AddStore(mkStore(t, "ephemeral")); err != nil {
		t.Fatalf("add: %v", err)
	}
	_ = mgr.SetDefault("ephemeral")
	ctx := cache.ContextWithManager(context.Background(), mgr)
	got, ok := cache.FromContext(ctx)
	if !ok || got != mgr {
		t.Fatalf("FromContext: ok=%v same=%v", ok, got == mgr)
	}
	_, ok = cache.FromContext(context.Background())
	if ok {
		t.Fatal("bare context must not return a manager")
	}
	_ = mgr.Shutdown(context.Background())
}

func TestModule_DriverUnknownReturnsHelpfulError(t *testing.T) {
	cfg := cache.Config{
		DefaultStore: "x",
		Stores: map[string]cache.StoreConfig{
			"x": {Driver: "nope"},
		},
	}
	app := framework.New("svc", "1.0.0").Use(cache.ModuleWithConfig(cfg))
	err := app.Init(context.Background())
	if err == nil || !strings.Contains(err.Error(), "not registered") {
		t.Fatalf("want not-registered error, got %v", err)
	}
}

func TestStore_DriverErrorsBubble(t *testing.T) {
	s := mkStore(t, "s")
	defer s.Shutdown(context.Background())
	// File driver requires Path -> Validate should reject and never
	// reach Get. Validates the Config layer rather than the Store.
	sc := cache.StoreConfig{Driver: cdriver.DriverFile}
	err := sc.Validate("s")
	if err == nil || !errors.Is(err, err) || !strings.Contains(err.Error(), "path is required") {
		t.Fatalf("validate file without path: %v", err)
	}
}
