package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/godx-jp/godx-platform-framework/framework"
)

func TestModuleWiresIntoApp(t *testing.T) {
	cfg := Config{Enabled: false}
	app := framework.New("svc", "0.0.0").Use(ModuleWithConfig(cfg, nil))
	ctx := context.Background()
	if err := app.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer func() { _ = app.Shutdown(ctx) }()

	sched, err := FromApp(app)
	if err != nil {
		t.Fatalf("FromApp: %v", err)
	}
	if err := sched.EveryMinute().Do("ping", func(ctx context.Context) error { return nil }); err != nil {
		t.Fatalf("Do: %v", err)
	}
}

func TestModuleDuplicateInitRejected(t *testing.T) {
	app := framework.New("svc", "0.0.0").Use(Module).Use(Module)
	ctx := context.Background()
	if err := app.Init(ctx); err == nil {
		t.Fatalf("expected duplicate init error")
	}
}

func TestModuleContextHelpers(t *testing.T) {
	sched := New(Options{})
	ctx := ContextWithScheduler(context.Background(), sched)
	got, ok := FromContext(ctx)
	if !ok || got != sched {
		t.Fatalf("ContextWithScheduler round trip failed")
	}
}

func TestLoadConfigFromEnv(t *testing.T) {
	t.Setenv(EnvEnabled, "false")
	t.Setenv(EnvLockTTL, "10m")
	t.Setenv(EnvLockPref, "lock:")
	cfg := LoadConfigFromEnv()
	if cfg.Enabled || cfg.LockTTL != 10*time.Minute || cfg.LockPrefix != "lock:" {
		t.Fatalf("env loading wrong: %+v", cfg)
	}
}
