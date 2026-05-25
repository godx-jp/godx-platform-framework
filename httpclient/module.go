package httpclient

import (
	"context"
	"fmt"

	"github.com/godx-jp/godx-platform-framework/framework"
	hdriver "github.com/godx-jp/godx-platform-framework/httpclient/driver"
)

const StoreKey = "godx.httpclient.manager"

var Module framework.Module = envModule{}

type envModule struct{}

func (envModule) Name() string { return "httpclient" }

func (envModule) Init(ctx context.Context, app *framework.App) error {
	return initFromConfig(ctx, app, LoadConfigFromEnv())
}

func ModuleWithConfig(cfg Config) framework.Module {
	return &fixedModule{cfg: cfg}
}

type fixedModule struct{ cfg Config }

func (f *fixedModule) Name() string { return "httpclient" }

func (f *fixedModule) Init(ctx context.Context, app *framework.App) error {
	return initFromConfig(ctx, app, f.cfg)
}

func initFromConfig(ctx context.Context, app *framework.App, cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if _, exists := app.Lookup(StoreKey); exists {
		return fmt.Errorf("httpclient: Module already initialised")
	}
	mgr := NewManager()
	for name, cc := range cfg.Clients {
		spec := cc.Spec
		if spec.Name == "" {
			spec.Name = cc.Driver
		}
		raw, err := hdriver.New(ctx, spec)
		if err != nil {
			return fmt.Errorf("httpclient: build %q: %w", name, err)
		}
		c := WrapWithBase(raw, spec.BaseURL)
		if err := mgr.AddClient(name, c); err != nil {
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
