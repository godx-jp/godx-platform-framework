package events

import (
	"context"
	"fmt"

	"github.com/godx-jp/godx-platform-framework/framework"
)

// StoreKey is the framework Store key under which the events Bus is
// published.
const StoreKey = "godx.events.bus"

// Module is the default events module. It loads configuration from
// the environment, constructs a synchronous Bus (optionally wrapped
// in async.New), and publishes it into the framework Store.
//
//	app := framework.New("svc", "1.0.0").Use(events.Module)
//	if err := app.Init(ctx); err != nil { return err }
//	bus, _ := events.FromApp(app)
//	bus.Listen("user.*", auditListener)
var Module framework.Module = envModule{}

type envModule struct{}

func (envModule) Name() string { return "events" }

func (envModule) Init(ctx context.Context, app *framework.App) error {
	return initFromConfig(ctx, app, LoadConfigFromEnv(), nil)
}

// ModuleWithConfig returns a framework.Module that uses the supplied
// Config instead of loading from env. onError is an optional sink
// for listener errors when Async is true.
func ModuleWithConfig(cfg Config, onError func(error)) framework.Module {
	return &fixedModule{cfg: cfg, onError: onError}
}

type fixedModule struct {
	cfg     Config
	onError func(error)
}

func (f *fixedModule) Name() string { return "events" }

func (f *fixedModule) Init(ctx context.Context, app *framework.App) error {
	return initFromConfig(ctx, app, f.cfg, f.onError)
}

func initFromConfig(ctx context.Context, app *framework.App, cfg Config, onError func(error)) error {
	if _, exists := app.Lookup(StoreKey); exists {
		return fmt.Errorf("events: Module already initialised (only one events.Module per App)")
	}
	bus := New()
	if cfg.Async {
		bus = NewAsync(bus, AsyncOptions{
			Workers:   cfg.AsyncWorkers,
			QueueSize: cfg.AsyncQueueSize,
			OnError:   onError,
		})
	}
	app.Store(StoreKey, bus)
	app.OnShutdown(bus.Close)
	return nil
}
