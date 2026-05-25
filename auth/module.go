package auth

import (
	"context"
	"fmt"

	"github.com/godx-jp/godx-platform-framework/framework"
	adriver "github.com/godx-jp/godx-platform-framework/auth/driver"
)

const StoreKey = "godx.auth.manager"

var Module framework.Module = envModule{}

type envModule struct{}

func (envModule) Name() string { return "auth" }

func (envModule) Init(ctx context.Context, app *framework.App) error {
	return initFromConfig(ctx, app, LoadConfigFromEnv())
}

func ModuleWithConfig(cfg Config) framework.Module {
	return &fixedModule{cfg: cfg}
}

type fixedModule struct{ cfg Config }

func (f *fixedModule) Name() string { return "auth" }

func (f *fixedModule) Init(ctx context.Context, app *framework.App) error {
	return initFromConfig(ctx, app, f.cfg)
}

func initFromConfig(ctx context.Context, app *framework.App, cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if _, exists := app.Lookup(StoreKey); exists {
		return fmt.Errorf("auth: Module already initialised")
	}
	mgr := NewManager()
	for name, gc := range cfg.Guards {
		spec := gc.Spec
		if spec.Name == "" {
			spec.Name = gc.Driver
		}
		g, err := adriver.New(ctx, spec)
		if err != nil {
			return fmt.Errorf("auth: build guard %q: %w", name, err)
		}
		if err := mgr.AddGuard(name, g); err != nil {
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
