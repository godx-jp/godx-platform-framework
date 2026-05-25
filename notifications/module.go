package notifications

import (
	"context"
	"fmt"

	"github.com/godx-jp/godx-platform-framework/events"
	"github.com/godx-jp/godx-platform-framework/framework"
	"github.com/godx-jp/godx-platform-framework/httpclient"
	"github.com/godx-jp/godx-platform-framework/mail"
)

// StoreKey is the framework Store key under which the notifications
// Manager is published.
const StoreKey = "godx.notifications.manager"

// Module is the default notifications module — reads config from env.
var Module framework.Module = envModule{}

type envModule struct{}

func (envModule) Name() string { return "notifications" }

func (envModule) Init(ctx context.Context, app *framework.App) error {
	return initFromConfig(ctx, app, LoadConfigFromEnv())
}

// ModuleWithConfig returns a framework.Module that uses the supplied
// Config instead of loading from env.
func ModuleWithConfig(cfg Config) framework.Module {
	return &fixedModule{cfg: cfg}
}

type fixedModule struct{ cfg Config }

func (f *fixedModule) Name() string { return "notifications" }

func (f *fixedModule) Init(ctx context.Context, app *framework.App) error {
	return initFromConfig(ctx, app, f.cfg)
}

func initFromConfig(ctx context.Context, app *framework.App, cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if _, exists := app.Lookup(StoreKey); exists {
		return fmt.Errorf("notifications: Module already initialised")
	}
	deps := buildDeps{databaseStore: cfg.DatabaseStore}
	if mgr, err := mail.FromApp(app); err == nil {
		deps.mailMgr = mgr
	}
	if hc, err := httpclient.FromApp(app); err == nil {
		if d := hc.Default(); d != nil {
			deps.httpClient = d
		}
	}
	mgr := NewManager()
	if bus, err := events.FromApp(app); err == nil {
		mgr.SetBus(bus)
	}
	for name, cc := range cfg.Channels {
		ch, err := buildChannel(ctx, cc, deps)
		if err != nil {
			return fmt.Errorf("notifications: build %q: %w", name, err)
		}
		if err := mgr.AddChannel(name, ch); err != nil {
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
