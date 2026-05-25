package cache

import (
	"context"
	"fmt"

	cdriver "github.com/godx-jp/godx-platform-framework/cache/driver"
	"github.com/godx-jp/godx-platform-framework/framework"
)

// StoreKey is the framework Store key under which the cache Manager
// is published. Exported so callers that prefer raw app.Lookup over
// the FromApp helper can opt in.
const StoreKey = "godx.cache.manager"

// Module is the default cache module. It loads configuration from
// the environment (LoadConfigFromEnv), constructs every configured
// store, validates the result, and publishes a *Manager into the
// framework Store under StoreKey.
//
//	app := framework.New("svc", "1.0.0").Use(cache.Module)
//	if err := app.Init(ctx); err != nil { return err }
//	mgr, _ := cache.FromApp(app)
//	_ = mgr.Default().Put(ctx, "k", []byte("v"), time.Minute)
var Module framework.Module = envModule{}

type envModule struct{}

func (envModule) Name() string { return "cache" }

func (envModule) Init(ctx context.Context, app *framework.App) error {
	return initFromConfig(ctx, app, LoadConfigFromEnv())
}

func initFromConfig(ctx context.Context, app *framework.App, cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if _, exists := app.Lookup(StoreKey); exists {
		return fmt.Errorf("cache: Module already initialised (only one cache.Module per App)")
	}
	mgr := NewManager()
	for name, sc := range cfg.Stores {
		store, err := buildStore(ctx, name, cfg.GlobalPrefix, sc)
		if err != nil {
			_ = mgr.Shutdown(ctx)
			return fmt.Errorf("cache: build store %q: %w", name, err)
		}
		if err := mgr.AddStore(store); err != nil {
			_ = mgr.Shutdown(ctx)
			return err
		}
	}
	if err := mgr.SetDefault(cfg.DefaultStore); err != nil {
		_ = mgr.Shutdown(ctx)
		return err
	}
	app.Store(StoreKey, mgr)
	app.OnShutdown(mgr.Shutdown)
	return nil
}

func buildStore(ctx context.Context, name, globalPrefix string, sc StoreConfig) (*Store, error) {
	spec := cdriver.Spec{
		Name:       sc.Driver,
		Prefix:     globalPrefix + sc.Prefix,
		DefaultTTL: sc.DefaultTTL,
		Path:       sc.Path,
		URL:        sc.URL,
		Address:    sc.Address,
		Username:   sc.Username,
		Password:   sc.Password,
		DB:         sc.DB,
		TLS:        sc.TLS,
	}
	drv, err := cdriver.New(ctx, spec)
	if err != nil {
		return nil, err
	}
	return NewStore(name, drv, sc), nil
}

// ModuleWithConfig returns a framework.Module that uses the supplied
// Config instead of loading from env. Useful for tests and code-driven
// configuration. Cannot be combined with Module on the same App.
func ModuleWithConfig(cfg Config) framework.Module {
	return &fixedModule{cfg: cfg}
}

type fixedModule struct{ cfg Config }

func (f *fixedModule) Name() string { return "cache" }

func (f *fixedModule) Init(ctx context.Context, app *framework.App) error {
	return initFromConfig(ctx, app, f.cfg)
}

// AddStore returns a framework.Module that registers a single
// additional store on top of the manager created by cache.Module.
// Must be wired AFTER cache.Module via app.Use.
func AddStore(name string, sc StoreConfig) framework.Module {
	return &addStoreMod{name: name, cfg: sc}
}

type addStoreMod struct {
	name string
	cfg  StoreConfig
}

func (a *addStoreMod) Name() string { return "cache.AddStore[" + a.name + "]" }

func (a *addStoreMod) Init(ctx context.Context, app *framework.App) error {
	v, ok := app.Lookup(StoreKey)
	if !ok {
		return fmt.Errorf("cache.AddStore(%q): Module must be wired before AddStore", a.name)
	}
	mgr, ok := v.(*Manager)
	if !ok {
		return fmt.Errorf("cache.AddStore(%q): %s key is not a *Manager", a.name, StoreKey)
	}
	if err := a.cfg.Validate(a.name); err != nil {
		return err
	}
	// AddStore via this path does not have the parent module's global
	// prefix in scope; pass the per-store prefix verbatim. Callers
	// wanting global-prefix composition should bake it into sc.Prefix.
	store, err := buildStore(ctx, a.name, "", a.cfg)
	if err != nil {
		return err
	}
	return mgr.AddStore(store)
}
