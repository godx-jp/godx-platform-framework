package cache_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/godx-jp/godx-platform-framework/cache"
	cdriver "github.com/godx-jp/godx-platform-framework/cache/driver"
	"github.com/godx-jp/godx-platform-framework/framework"
)

// ────────────────────────────────────────────────────────────────────
//                         Manager edges
// ────────────────────────────────────────────────────────────────────

func TestManager_AddStore_NilRejected(t *testing.T) {
	mgr := cache.NewManager()
	if err := mgr.AddStore(nil); err == nil {
		t.Fatal("AddStore(nil) should error")
	}
}

func TestManager_AddStore_EmptyNameRejected(t *testing.T) {
	mgr := cache.NewManager()
	drv, _ := cdriver.New(context.Background(), cdriver.Spec{Name: cdriver.DriverMemory})
	t.Cleanup(func() { _ = drv.Shutdown(context.Background()) })
	if err := mgr.AddStore(cache.NewStore("", drv, cache.StoreConfig{Driver: cdriver.DriverMemory})); err == nil {
		t.Fatal("AddStore with empty name should error")
	}
}

func TestManager_SetDefault_UnknownStoreRejected(t *testing.T) {
	mgr := cache.NewManager()
	if err := mgr.SetDefault("ghost"); err == nil {
		t.Fatal("SetDefault on missing store should error")
	}
}

func TestManager_DefaultPanicsWhenUnset(t *testing.T) {
	mgr := cache.NewManager()
	defer func() {
		if recover() == nil {
			t.Fatal("Default() before SetDefault must panic")
		}
	}()
	_ = mgr.Default()
}

func TestManager_MustStorePanicsOnMissing(t *testing.T) {
	mgr := cache.NewManager()
	defer func() {
		if recover() == nil {
			t.Fatal("MustStore must panic on missing")
		}
	}()
	_ = mgr.MustStore("ghost")
}

func TestManager_ShutdownIsIdempotent(t *testing.T) {
	mgr := cache.NewManager()
	drv, _ := cdriver.New(context.Background(), cdriver.Spec{Name: cdriver.DriverMemory})
	_ = mgr.AddStore(cache.NewStore("s", drv, cache.StoreConfig{Driver: cdriver.DriverMemory}))
	_ = mgr.SetDefault("s")

	if err := mgr.Shutdown(context.Background()); err != nil {
		t.Fatalf("1st: %v", err)
	}
	if err := mgr.Shutdown(context.Background()); err != nil {
		t.Fatalf("2nd: %v", err)
	}
	// After Shutdown, Store list is empty.
	if got := mgr.Stores(); len(got) != 0 {
		t.Fatalf("Stores() after Shutdown = %v; want empty", got)
	}
	if got := mgr.DefaultName(); got != "" {
		t.Fatalf("DefaultName() after Shutdown = %q; want empty", got)
	}
}

func TestManager_StoresReturnedSorted(t *testing.T) {
	mgr := cache.NewManager()
	defer mgr.Shutdown(context.Background())
	for _, n := range []string{"zebra", "alpha", "middle"} {
		drv, _ := cdriver.New(context.Background(), cdriver.Spec{Name: cdriver.DriverMemory})
		if err := mgr.AddStore(cache.NewStore(n, drv, cache.StoreConfig{Driver: cdriver.DriverMemory})); err != nil {
			t.Fatalf("add %s: %v", n, err)
		}
	}
	got := mgr.Stores()
	prev := ""
	for _, n := range got {
		if n < prev {
			t.Fatalf("Stores() not sorted: %v", got)
		}
		prev = n
	}
}

// ────────────────────────────────────────────────────────────────────
//                          Module edges
// ────────────────────────────────────────────────────────────────────

func TestModule_DoubleInitRejected(t *testing.T) {
	cfg := cache.Config{
		DefaultStore: "memory",
		Stores:       map[string]cache.StoreConfig{"memory": {Driver: cdriver.DriverMemory}},
	}
	app := framework.New("svc", "1.0.0").
		Use(cache.ModuleWithConfig(cfg)).
		Use(cache.ModuleWithConfig(cfg))
	err := app.Init(context.Background())
	if err == nil || !strings.Contains(err.Error(), "already initialised") {
		t.Fatalf("want already-initialised error, got %v", err)
	}
}

func TestModule_AddStoreBeforeModuleRejected(t *testing.T) {
	app := framework.New("svc", "1.0.0").
		Use(cache.AddStore("late", cache.StoreConfig{Driver: cdriver.DriverMemory}))
	err := app.Init(context.Background())
	if err == nil || !strings.Contains(err.Error(), "Module must be wired before AddStore") {
		t.Fatalf("want module-before-addstore error, got %v", err)
	}
}

func TestModule_FromApp_BeforeInitErrors(t *testing.T) {
	app := framework.New("svc", "1.0.0")
	if _, err := cache.FromApp(app); err == nil {
		t.Fatal("FromApp before Init should error")
	}
}

func TestModule_GlobalPrefixComposesWithPerStorePrefix(t *testing.T) {
	cfg := cache.Config{
		DefaultStore: "primary",
		GlobalPrefix: "svc:",
		Stores: map[string]cache.StoreConfig{
			"primary": {Driver: cdriver.DriverMemory, Prefix: "primary:"},
			"audit":   {Driver: cdriver.DriverMemory, Prefix: "audit:"},
		},
	}
	app := framework.New("svc", "1.0.0").Use(cache.ModuleWithConfig(cfg))
	if err := app.Init(context.Background()); err != nil {
		t.Fatalf("init: %v", err)
	}
	defer app.Shutdown(context.Background())

	mgr, _ := cache.FromApp(app)
	// Put through one store, look up through another with the same logical key — must be isolated.
	_ = mgr.MustStore("primary").Put(context.Background(), "k", []byte("p"), 0)
	_ = mgr.MustStore("audit").Put(context.Background(), "k", []byte("a"), 0)

	v, _, _ := mgr.MustStore("primary").Get(context.Background(), "k")
	if string(v) != "p" {
		t.Fatalf("primary read leaked: %q", v)
	}
	v, _, _ = mgr.MustStore("audit").Get(context.Background(), "k")
	if string(v) != "a" {
		t.Fatalf("audit read leaked: %q", v)
	}
}

func TestModule_BuildFailureDoesNotLeakStores(t *testing.T) {
	cfg := cache.Config{
		DefaultStore: "ok",
		Stores: map[string]cache.StoreConfig{
			"ok":   {Driver: cdriver.DriverMemory},
			"bad":  {Driver: "ghost-driver-that-does-not-exist"},
			"file": {Driver: cdriver.DriverFile, Path: t.TempDir()},
		},
	}
	app := framework.New("svc", "1.0.0").Use(cache.ModuleWithConfig(cfg))
	err := app.Init(context.Background())
	if err == nil {
		t.Fatal("want error from unknown driver")
	}
	// FromApp should also error — Module aborted, nothing in the
	// framework Store.
	if _, ferr := cache.FromApp(app); ferr == nil {
		t.Fatal("FromApp after failed Init should error")
	}
}

// ────────────────────────────────────────────────────────────────────
//                         Config edges
// ────────────────────────────────────────────────────────────────────

func TestConfig_EnvSegmentNormalisationHandlesCornerCases(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  string
		want string
	}{
		{"user-sessions", "CACHE_STORE_USER_SESSIONS_DRIVER", cdriver.DriverMemory},
		{"User-Sessions", "CACHE_STORE_USER_SESSIONS_DRIVER", cdriver.DriverMemory},
		{"weird name!@#", "CACHE_STORE_WEIRDNAME_DRIVER", cdriver.DriverMemory},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(tc.env, "memory")
			sc := cache.LoadStoreConfigFromEnv(tc.name)
			if sc.Driver != tc.want {
				t.Fatalf("driver = %q, want %q", sc.Driver, tc.want)
			}
		})
	}
}

func TestConfig_DefaultMustBeListed(t *testing.T) {
	t.Setenv(cache.EnvDefaultStore, "ghost")
	t.Setenv(cache.EnvStores, "real")
	t.Setenv("CACHE_STORE_REAL_DRIVER", "memory")
	cfg := cache.LoadConfigFromEnv()
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "default store") {
		t.Fatalf("want default-not-in-stores error, got %v", err)
	}
}

func TestConfig_EmptyStoresIsInvalid(t *testing.T) {
	cfg := cache.Config{DefaultStore: "x", Stores: nil}
	if err := cfg.Validate(); err == nil {
		t.Fatal("empty Stores must fail Validate")
	}
}

func TestConfig_DriverEmptyOnExplicitStoreIsRejected(t *testing.T) {
	cfg := cache.Config{
		DefaultStore: "x",
		Stores:       map[string]cache.StoreConfig{"x": {}},
	}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "driver is required") {
		t.Fatalf("want driver-required error, got %v", err)
	}
}

// ────────────────────────────────────────────────────────────────────
//                          Store edges
// ────────────────────────────────────────────────────────────────────

func TestStore_PutWithNegativeTTLBehavesLikeForever(t *testing.T) {
	s := newMemoryStore(t)
	ctx := context.Background()
	if err := s.Put(ctx, "k", []byte("v"), -5*time.Second); err != nil {
		t.Fatalf("put neg ttl: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	if _, ok, _ := s.Get(ctx, "k"); !ok {
		t.Fatal("negative TTL should clamp to forever, not insta-expire")
	}
}

func TestStore_AddWithNegativeTTLClamps(t *testing.T) {
	s := newMemoryStore(t)
	ctx := context.Background()
	ok, err := s.Add(ctx, "k", []byte("v"), -time.Hour)
	if err != nil || !ok {
		t.Fatalf("add neg ttl: ok=%v err=%v", ok, err)
	}
}

func TestStore_PullOnMissingIsNoop(t *testing.T) {
	s := newMemoryStore(t)
	ctx := context.Background()
	if _, ok, err := s.Pull(ctx, "ghost"); err != nil || ok {
		t.Fatalf("pull missing: ok=%v err=%v", ok, err)
	}
}

func TestStore_RememberJSON_HitDoesNotCallFn(t *testing.T) {
	s := newMemoryStore(t)
	ctx := context.Background()
	type T struct {
		N int `json:"n"`
	}
	if err := s.PutJSON(ctx, "k", T{N: 7}, 0); err != nil {
		t.Fatalf("putjson: %v", err)
	}
	calls := 0
	var out T
	err := s.RememberJSON(ctx, "k", time.Minute, &out, func(context.Context) (any, error) {
		calls++
		return T{N: 99}, nil
	})
	if err != nil || out.N != 7 || calls != 0 {
		t.Fatalf("hit: out=%+v calls=%d err=%v", out, calls, err)
	}
}

func TestStore_GetJSON_OnCorruptValue(t *testing.T) {
	s := newMemoryStore(t)
	ctx := context.Background()
	// Put non-JSON bytes via the Store's bytes API, then read via GetJSON.
	_ = s.Put(ctx, "broken", []byte("not-json{"), 0)
	type T struct {
		X int `json:"x"`
	}
	var v T
	ok, err := s.GetJSON(ctx, "broken", &v)
	if err == nil {
		t.Fatal("GetJSON over invalid bytes must error")
	}
	// ok must still be true so the caller can distinguish "miss" from
	// "hit but corrupt" — matches the Store contract.
	if !ok {
		t.Fatal("GetJSON corrupt: ok should be true (we did find a value)")
	}
}

func TestStore_NameAndDriverExposed(t *testing.T) {
	drv, _ := cdriver.New(context.Background(), cdriver.Spec{Name: cdriver.DriverMemory})
	s := cache.NewStore("auth", drv, cache.StoreConfig{Driver: cdriver.DriverMemory})
	if s.Name() != "auth" {
		t.Fatalf("Name(): %q", s.Name())
	}
	if s.Driver() != drv {
		t.Fatal("Driver() did not return the constructed driver")
	}
	if s.Config().Driver != cdriver.DriverMemory {
		t.Fatalf("Config(): %+v", s.Config())
	}
	_ = s.Shutdown(context.Background())
}

// ────────────────────────────────────────────────────────────────────
//                    Concurrent Manager use
// ────────────────────────────────────────────────────────────────────

func TestManager_ConcurrentLookupSafe(t *testing.T) {
	mgr := cache.NewManager()
	defer mgr.Shutdown(context.Background())
	for _, n := range []string{"a", "b", "c", "d"} {
		drv, _ := cdriver.New(context.Background(), cdriver.Spec{Name: cdriver.DriverMemory})
		if err := mgr.AddStore(cache.NewStore(n, drv, cache.StoreConfig{Driver: cdriver.DriverMemory})); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	_ = mgr.SetDefault("a")

	const readers = 32
	var wg sync.WaitGroup
	wg.Add(readers)
	for i := 0; i < readers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 500; j++ {
				_ = mgr.Default()
				_, _ = mgr.Store("b")
				_ = mgr.Stores()
				_ = mgr.DefaultName()
			}
		}()
	}
	wg.Wait()
}
