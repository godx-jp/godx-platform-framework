package storage

import (
	"context"
	"fmt"

	"github.com/godx-jp/godx-platform-framework/framework"
)

// StoreKey is the framework Store key under which the Manager is
// published. Exported so callers that prefer raw app.Lookup over the
// FromApp helper can opt in.
const StoreKey = "godx.storage.manager"

// Module is the default storage module. It loads configuration from the
// environment (LoadConfigFromEnv), constructs every configured disk,
// validates the result, and publishes a *Manager into the framework
// Store under StoreKey.
//
// Add it to an App via framework.New(...).Use(storage.Module):
//
//	app := framework.New("svc", "1.0.0").Use(storage.Module)
//	if err := app.Init(ctx); err != nil { return err }
//	mgr, _ := storage.FromApp(app)
//	disk, _ := mgr.Disk("local")
var Module framework.Module = envModule{}

type envModule struct{}

func (envModule) Name() string { return "storage" }

func (envModule) Init(ctx context.Context, app *framework.App) error {
	return initFromConfig(ctx, app, LoadConfigFromEnv())
}

func initFromConfig(ctx context.Context, app *framework.App, cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if _, exists := app.Lookup(StoreKey); exists {
		return fmt.Errorf("storage: Module already initialised (only one storage.Module per App)")
	}
	mgr := NewManager()
	for name, dc := range cfg.Disks {
		d, err := buildDisk(ctx, name, dc)
		if err != nil {
			_ = mgr.Shutdown(ctx)
			return fmt.Errorf("storage: build disk %q: %w", name, err)
		}
		if err := mgr.AddDisk(name, d); err != nil {
			_ = mgr.Shutdown(ctx)
			return err
		}
	}
	if err := mgr.SetDefault(cfg.DefaultDisk); err != nil {
		_ = mgr.Shutdown(ctx)
		return err
	}
	app.Store(StoreKey, mgr)
	app.OnShutdown(mgr.Shutdown)
	return nil
}

// ModuleWithConfig returns a framework.Module that uses the supplied
// Config instead of loading from env. Useful for tests and code-driven
// configuration. Cannot be combined with Module on the same App.
func ModuleWithConfig(cfg Config) framework.Module {
	return &fixedModule{cfg: cfg}
}

type fixedModule struct{ cfg Config }

func (f *fixedModule) Name() string { return "storage" }

func (f *fixedModule) Init(ctx context.Context, app *framework.App) error {
	return initFromConfig(ctx, app, f.cfg)
}

// AddDisk returns a framework.Module that registers a single additional
// disk on top of the manager created by storage.Module. Must be wired
// AFTER storage.Module via app.Use.
func AddDisk(name string, cfg DiskConfig) framework.Module {
	return &addDiskMod{name: name, cfg: cfg}
}

type addDiskMod struct {
	name string
	cfg  DiskConfig
}

func (a *addDiskMod) Name() string { return "storage.AddDisk[" + a.name + "]" }

func (a *addDiskMod) Init(ctx context.Context, app *framework.App) error {
	v, ok := app.Lookup(StoreKey)
	if !ok {
		return fmt.Errorf("storage.AddDisk(%q): Module must be wired before AddDisk", a.name)
	}
	mgr, ok := v.(*Manager)
	if !ok {
		return fmt.Errorf("storage.AddDisk(%q): %s key is not a *Manager", a.name, StoreKey)
	}
	if err := a.cfg.Validate(a.name); err != nil {
		return err
	}
	d, err := buildDisk(ctx, a.name, a.cfg)
	if err != nil {
		return err
	}
	return mgr.AddDisk(a.name, d)
}
