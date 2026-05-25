package scheduler

import (
	"context"
	"fmt"

	"github.com/godx-jp/godx-platform-framework/framework"
	"github.com/godx-jp/godx-platform-framework/scheduler/lock"
)

// StoreKey is the framework Store key under which the Scheduler is published.
const StoreKey = "godx.scheduler"

// Module is the default scheduler module.
var Module framework.Module = envModule{}

type envModule struct{}

func (envModule) Name() string { return "scheduler" }

func (envModule) Init(ctx context.Context, app *framework.App) error {
	return initFromConfig(ctx, app, LoadConfigFromEnv(), nil)
}

// ModuleWithConfig returns a framework.Module using the supplied Config.
// cacheLock is optional; pass nil when OnOneServer is unused.
func ModuleWithConfig(cfg Config, cacheLock lock.CacheStore) framework.Module {
	return &fixedModule{cfg: cfg, cacheLock: cacheLock}
}

type fixedModule struct {
	cfg       Config
	cacheLock lock.CacheStore
}

func (f *fixedModule) Name() string { return "scheduler" }

func (f *fixedModule) Init(ctx context.Context, app *framework.App) error {
	return initFromConfig(ctx, app, f.cfg, f.cacheLock)
}

func initFromConfig(ctx context.Context, app *framework.App, cfg Config, cacheLock lock.CacheStore) error {
	cfg = cfg.withDefaults()
	if err := cfg.Validate(); err != nil {
		return err
	}
	if _, exists := app.Lookup(StoreKey); exists {
		return fmt.Errorf("scheduler: Module already initialised (only one scheduler.Module per App)")
	}
	opts := Options{}
	if cacheLock != nil {
		cl, err := lock.NewCache(lock.CacheOptions{
			Store:  cacheLock,
			Prefix: cfg.LockPrefix,
			Owner:  app.Name(),
			TTL:    cfg.LockTTL,
		})
		if err != nil {
			return fmt.Errorf("scheduler: cache lock: %w", err)
		}
		opts.DistributedLock = cl
	}
	sched := New(opts)
	app.Store(StoreKey, sched)
	app.OnShutdown(sched.Stop)
	if cfg.Enabled {
		if err := sched.Start(ctx); err != nil {
			return fmt.Errorf("scheduler: start: %w", err)
		}
	}
	return nil
}
