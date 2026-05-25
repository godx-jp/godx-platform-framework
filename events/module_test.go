package events

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/godx-jp/godx-platform-framework/framework"
)

func TestModuleWiresIntoApp(t *testing.T) {
	app := framework.New("svc", "0.0.0").Use(Module)
	ctx := context.Background()
	if err := app.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer func() { _ = app.Shutdown(ctx) }()

	bus, err := FromApp(app)
	if err != nil {
		t.Fatalf("FromApp: %v", err)
	}
	var n atomic.Int32
	bus.Listen("x", func(ctx context.Context, e Event) error { n.Add(1); return nil })
	_ = bus.Dispatch(ctx, Event{Name: "x"})
	if n.Load() != 1 {
		t.Fatalf("dispatch did not land")
	}
}

func TestModuleAsyncFromConfig(t *testing.T) {
	cfg := Config{Async: true, AsyncWorkers: 1, AsyncQueueSize: 16}
	app := framework.New("svc", "0.0.0").Use(ModuleWithConfig(cfg, nil))
	ctx := context.Background()
	if err := app.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer func() { _ = app.Shutdown(ctx) }()
	bus, _ := FromApp(app)
	if bus == nil {
		t.Fatalf("Bus nil")
	}
}

func TestModuleContextHelpers(t *testing.T) {
	bus := New()
	defer func() { _ = bus.Close(context.Background()) }()
	ctx := ContextWithBus(context.Background(), bus)
	got, ok := FromContext(ctx)
	if !ok || got != bus {
		t.Fatalf("ContextWithBus round trip failed")
	}
	if _, ok := FromContext(context.Background()); ok {
		t.Fatalf("FromContext on plain ctx should be false")
	}
	if _, ok := FromContext(nil); ok {
		t.Fatalf("FromContext on nil ctx should be false")
	}
}

func TestLoadConfigFromEnv(t *testing.T) {
	t.Setenv(EnvAsync, "true")
	t.Setenv(EnvAsyncWorkers, "8")
	t.Setenv(EnvAsyncQueueSize, "1024")
	cfg := LoadConfigFromEnv()
	if !cfg.Async || cfg.AsyncWorkers != 8 || cfg.AsyncQueueSize != 1024 {
		t.Fatalf("env loading wrong: %+v", cfg)
	}
}
