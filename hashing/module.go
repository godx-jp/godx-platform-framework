package hashing

import (
	"context"
	"fmt"

	hdriver "github.com/godx-jp/godx-platform-framework/hashing/driver"
	"github.com/godx-jp/godx-platform-framework/framework"
)

// StoreKey is the framework Store key under which the hashing Manager
// is published.
const StoreKey = "godx.hashing.manager"

// Module is the default hashing module.
var Module framework.Module = envModule{}

type envModule struct{}

func (envModule) Name() string { return "hashing" }

func (envModule) Init(ctx context.Context, app *framework.App) error {
	return initFromConfig(ctx, app, LoadConfigFromEnv())
}

// ModuleWithConfig returns a framework.Module that uses the supplied
// Config instead of loading from env.
func ModuleWithConfig(cfg Config) framework.Module {
	return &fixedModule{cfg: cfg}
}

type fixedModule struct{ cfg Config }

func (f *fixedModule) Name() string { return "hashing" }

func (f *fixedModule) Init(ctx context.Context, app *framework.App) error {
	return initFromConfig(ctx, app, f.cfg)
}

func initFromConfig(ctx context.Context, app *framework.App, cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if _, exists := app.Lookup(StoreKey); exists {
		return fmt.Errorf("hashing: Module already initialised (only one hashing.Module per App)")
	}
	mgr := NewManager()
	for name, hc := range cfg.Hashers {
		spec := hc.Spec
		if spec.Name == "" {
			spec.Name = hc.Driver
		}
		h, err := hdriver.New(ctx, spec)
		if err != nil {
			return fmt.Errorf("hashing: build hasher %q: %w", name, err)
		}
		if err := mgr.AddHasher(name, h); err != nil {
			return err
		}
	}
	if err := mgr.SetDefault(cfg.Default); err != nil {
		return err
	}
	app.Store(StoreKey, mgr)
	app.OnShutdown(mgr.Shutdown)
	return nil
}

// MustDefault constructs the default hasher (bcrypt with cost 12)
// without going through the module. Handy for tests and scripts.
func MustDefault() hdriver.Hasher {
	h, err := hdriver.New(context.Background(), hdriver.Spec{Name: hdriver.DriverBcrypt})
	if err != nil {
		panic(err)
	}
	return h
}
