package mail

import (
	"context"
	"fmt"

	"github.com/godx-jp/godx-platform-framework/events"
	"github.com/godx-jp/godx-platform-framework/framework"
	mdriver "github.com/godx-jp/godx-platform-framework/mail/driver"
)

// StoreKey is the framework Store key under which the mail Manager
// is published.
const StoreKey = "godx.mail.manager"

// Module is the default mail module — reads config from env.
var Module framework.Module = envModule{}

type envModule struct{}

func (envModule) Name() string { return "mail" }

func (envModule) Init(ctx context.Context, app *framework.App) error {
	return initFromConfig(ctx, app, LoadConfigFromEnv())
}

// ModuleWithConfig returns a framework.Module that uses the supplied
// Config instead of loading from env.
func ModuleWithConfig(cfg Config) framework.Module {
	return &fixedModule{cfg: cfg}
}

type fixedModule struct{ cfg Config }

func (f *fixedModule) Name() string { return "mail" }

func (f *fixedModule) Init(ctx context.Context, app *framework.App) error {
	return initFromConfig(ctx, app, f.cfg)
}

func initFromConfig(ctx context.Context, app *framework.App, cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if _, exists := app.Lookup(StoreKey); exists {
		return fmt.Errorf("mail: Module already initialised")
	}
	mgr := NewManager()
	mgr.SetDefaultFrom(cfg.From)
	if bus, err := events.FromApp(app); err == nil {
		mgr.SetBus(bus)
	}
	for name, mc := range cfg.Mailers {
		spec := mc.Spec
		if spec.Name == "" {
			spec.Name = mc.Driver
		}
		if spec.From == "" {
			spec.From = cfg.From
		}
		t, err := mdriver.New(ctx, spec)
		if err != nil {
			return fmt.Errorf("mail: build %q: %w", name, err)
		}
		if err := mgr.AddTransport(name, t); err != nil {
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
