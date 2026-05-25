package ratelimit

import (
	"context"
	"fmt"

	"github.com/godx-jp/godx-platform-framework/framework"
	rdriver "github.com/godx-jp/godx-platform-framework/ratelimit/driver"
)

const StoreKey = "godx.ratelimit.manager"

var Module framework.Module = envModule{}

type envModule struct{}

func (envModule) Name() string { return "ratelimit" }

func (envModule) Init(ctx context.Context, app *framework.App) error {
	return initFromConfig(ctx, app, LoadConfigFromEnv())
}

func ModuleWithConfig(cfg Config) framework.Module {
	return &fixedModule{cfg: cfg}
}

type fixedModule struct{ cfg Config }

func (f *fixedModule) Name() string { return "ratelimit" }

func (f *fixedModule) Init(ctx context.Context, app *framework.App) error {
	return initFromConfig(ctx, app, f.cfg)
}

func initFromConfig(ctx context.Context, app *framework.App, cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if _, exists := app.Lookup(StoreKey); exists {
		return fmt.Errorf("ratelimit: Module already initialised")
	}
	mgr := NewManager()
	for name, lc := range cfg.Limiters {
		spec := lc.Spec
		if spec.Name == "" {
			spec.Name = lc.Driver
		}
		lim, err := rdriver.New(ctx, spec)
		if err != nil {
			return fmt.Errorf("ratelimit: build %q: %w", name, err)
		}
		if err := mgr.AddLimiter(name, lim); err != nil {
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
