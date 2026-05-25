package config

import (
	"context"
	"fmt"

	cdriver "github.com/godx-jp/godx-platform-framework/config/driver"
	"github.com/godx-jp/godx-platform-framework/framework"
)

// StoreKey is the framework Store key under which the config Manager
// is published.
const StoreKey = "godx.config.manager"

// Module is the default config module. It loads configuration from
// the environment, constructs every configured source, and publishes
// a *Manager into the framework Store under StoreKey.
//
//	app := framework.New("svc", "1.0.0").Use(config.Module)
//	if err := app.Init(ctx); err != nil { return err }
//	cfg, _ := config.FromApp(app)
//	port := cfg.GetInt("server.port", 8080)
var Module framework.Module = envModule{}

type envModule struct{}

func (envModule) Name() string { return "config" }

func (envModule) Init(ctx context.Context, app *framework.App) error {
	return initFromConfig(ctx, app, LoadConfigFromEnv())
}

func initFromConfig(ctx context.Context, app *framework.App, cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if _, exists := app.Lookup(StoreKey); exists {
		return fmt.Errorf("config: Module already initialised (only one config.Module per App)")
	}
	mgr := NewManager()
	for _, ns := range cfg.Sources {
		src, err := cdriver.New(ctx, ns.Config.ToSpec(ns.Name))
		if err != nil {
			_ = mgr.Shutdown(ctx)
			return fmt.Errorf("config: build source %q: %w", ns.Name, err)
		}
		if err := mgr.AddSource(ctx, ns.Name, src); err != nil {
			_ = mgr.Shutdown(ctx)
			return err
		}
	}
	if cfg.AutoEnv {
		spec := cdriver.Spec{Name: cdriver.DriverEnv, Prefix: cfg.AutoEnvPrefix}
		src, err := cdriver.New(ctx, spec)
		if err != nil {
			_ = mgr.Shutdown(ctx)
			return fmt.Errorf("config: build auto-env source: %w", err)
		}
		if err := mgr.AddSource(ctx, "__auto_env__", src); err != nil {
			_ = mgr.Shutdown(ctx)
			return err
		}
	}
	app.Store(StoreKey, mgr)
	app.OnShutdown(mgr.Shutdown)
	return nil
}

// ModuleWithConfig returns a framework.Module that uses the supplied
// Config instead of loading from env. Useful for tests and
// code-driven configuration.
func ModuleWithConfig(cfg Config) framework.Module {
	return &fixedModule{cfg: cfg}
}

type fixedModule struct{ cfg Config }

func (f *fixedModule) Name() string { return "config" }

func (f *fixedModule) Init(ctx context.Context, app *framework.App) error {
	return initFromConfig(ctx, app, f.cfg)
}
