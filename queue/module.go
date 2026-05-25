package queue

import (
	"context"
	"fmt"

	"github.com/godx-jp/godx-platform-framework/events"
	"github.com/godx-jp/godx-platform-framework/framework"
	qdriver "github.com/godx-jp/godx-platform-framework/queue/driver"
)

const StoreKey = "godx.queue.manager"

var Module framework.Module = envModule{}

type envModule struct{}

func (envModule) Name() string { return "queue" }

func (envModule) Init(ctx context.Context, app *framework.App) error {
	return initFromConfig(ctx, app, LoadConfigFromEnv(), nil)
}

func ModuleWithConfig(cfg Config, bus events.Bus) framework.Module {
	return &fixedModule{cfg: cfg, bus: bus}
}

type fixedModule struct {
	cfg Config
	bus events.Bus
}

func (f *fixedModule) Name() string { return "queue" }

func (f *fixedModule) Init(ctx context.Context, app *framework.App) error {
	return initFromConfig(ctx, app, f.cfg, f.bus)
}

func initFromConfig(ctx context.Context, app *framework.App, cfg Config, bus events.Bus) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if _, exists := app.Lookup(StoreKey); exists {
		return fmt.Errorf("queue: Module already initialised")
	}
	mgr := NewManager()
	for name, qc := range cfg.Queues {
		q, err := buildQueue(ctx, name, qc, bus)
		if err != nil {
			_ = mgr.Shutdown(ctx)
			return fmt.Errorf("queue: build %q: %w", name, err)
		}
		if err := mgr.AddQueue(q); err != nil {
			_ = mgr.Shutdown(ctx)
			return err
		}
	}
	if err := mgr.SetDefault(cfg.Default); err != nil {
		_ = mgr.Shutdown(ctx)
		return err
	}
	app.Store(StoreKey, mgr)
	app.OnShutdown(mgr.Shutdown)
	return nil
}

func buildQueue(ctx context.Context, name string, qc QueueConfig, bus events.Bus) (*Queue, error) {
	spec := qdriver.Spec{
		Name:         qc.Driver,
		DefaultQueue: qc.DefaultQueue,
		Workers:      qc.Workers,
		QueueSize:    qc.QueueSize,
		AWSRegion:    qc.AWSRegion,
		QueueURL:     qc.QueueURL,
		Brokers:      qc.Brokers,
		Topic:        qc.Topic,
		GroupID:      qc.GroupID,
		NATSURL:      qc.NATSURL,
		Subject:      qc.Subject,
		StreamName:   qc.StreamName,
		URL:          qc.RedisURL,
		Address:      qc.RedisAddress,
		Prefix:       qc.RedisPrefix,
	}
	backend, err := qdriver.New(ctx, spec)
	if err != nil {
		return nil, err
	}
	opts := []Option{
		WithDefaultQueue(qc.DefaultQueue),
		WithWorkers(qc.Workers),
	}
	if bus != nil {
		opts = append(opts, WithBus(bus))
	}
	return NewQueue(name, backend, opts...), nil
}
