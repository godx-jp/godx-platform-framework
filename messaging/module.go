package messaging

import (
	"context"
	"fmt"

	"github.com/godx-jp/godx-platform-framework/events"
	"github.com/godx-jp/godx-platform-framework/framework"
	mdriver "github.com/godx-jp/godx-platform-framework/messaging/driver"
	"github.com/godx-jp/godx-platform-framework/messaging/envelope"
)

var Module framework.Module = envModule{}

type envModule struct{}

func (envModule) Name() string { return "messaging" }

func (envModule) Init(ctx context.Context, app *framework.App) error {
	return initFromConfig(ctx, app, LoadConfigFromEnv())
}

func ModuleWithConfig(cfg Config) framework.Module {
	return &fixedModule{cfg: cfg}
}

type fixedModule struct{ cfg Config }

func (f *fixedModule) Name() string { return "messaging" }

func (f *fixedModule) Init(ctx context.Context, app *framework.App) error {
	return initFromConfig(ctx, app, f.cfg)
}

func initFromConfig(ctx context.Context, app *framework.App, cfg Config) error {
	if _, ok := app.Lookup(StoreKey); ok {
		return fmt.Errorf("messaging: Module already initialised")
	}
	mgr := NewManager()
	for name, cc := range cfg.Connections {
		spec := cc.Driver
		if spec.Name == "" {
			spec.Name = mdriver.DriverMemory
		}
		b, err := mdriver.New(ctx, spec)
		if err != nil {
			return fmt.Errorf("messaging: connection %q: %w", name, err)
		}
		if err := mgr.Add(name, b); err != nil {
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

// ForwardListener publishes integration.* events to the broker.
func ForwardListener(pub *Publisher, source string) events.Listener {
	return func(ctx context.Context, e events.Event) error {
		id, _ := e.Metadata["id"]
		if id == "" {
			id = fmt.Sprintf("%s-%d", e.Name, e.CreatedAt.UnixNano())
		}
		return pub.Publish(ctx, envelope.Event{
			ID:     id,
			Source: source,
			Type:   e.Name,
			Data:   []byte(fmt.Sprintf("%v", e.Payload)),
		})
	}
}
