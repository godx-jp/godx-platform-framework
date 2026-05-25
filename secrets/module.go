package secrets

import (
	"context"
	"fmt"

	"github.com/godx-jp/godx-platform-framework/framework"
	sdriver "github.com/godx-jp/godx-platform-framework/secrets/driver"
)

// StoreKey is the framework Store key under which the secrets
// Manager is published.
const StoreKey = "godx.secrets.manager"

// Module is the default secrets module — reads config from env.
var Module framework.Module = envModule{}

type envModule struct{}

func (envModule) Name() string { return "secrets" }

func (envModule) Init(ctx context.Context, app *framework.App) error {
	return initFromConfig(ctx, app, LoadConfigFromEnv())
}

// ModuleWithConfig returns a framework.Module that uses the supplied
// Config instead of loading from env. Useful for tests and code-driven
// setup.
func ModuleWithConfig(cfg Config) framework.Module {
	return &fixedModule{cfg: cfg}
}

type fixedModule struct{ cfg Config }

func (f *fixedModule) Name() string { return "secrets" }

func (f *fixedModule) Init(ctx context.Context, app *framework.App) error {
	return initFromConfig(ctx, app, f.cfg)
}

func initFromConfig(ctx context.Context, app *framework.App, cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if _, exists := app.Lookup(StoreKey); exists {
		return fmt.Errorf("secrets: Module already initialised (only one secrets.Module per App)")
	}
	mgr := NewManager()
	for name, sc := range cfg.Stores {
		spec := sc.Spec
		if spec.Name == "" {
			spec.Name = sc.Driver
		}
		st, err := sdriver.New(ctx, spec)
		if err != nil {
			return fmt.Errorf("secrets: build store %q: %w", name, err)
		}
		if err := mgr.AddStore(name, st); err != nil {
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
