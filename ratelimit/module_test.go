package ratelimit

import (
	"context"
	"testing"

	"github.com/godx-jp/godx-platform-framework/framework"
	rdriver "github.com/godx-jp/godx-platform-framework/ratelimit/driver"
)

func TestModuleWires(t *testing.T) {
	cfg := Config{
		Default: rdriver.DriverMemory,
		Limiters: map[string]LimiterConfig{
			rdriver.DriverMemory: {
				Driver: rdriver.DriverMemory,
				Spec:   rdriver.Spec{Name: rdriver.DriverMemory, Rate: 10, Burst: 5},
			},
		},
	}
	app := framework.New("svc", "0.0.0").Use(ModuleWithConfig(cfg))
	if err := app.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer app.Shutdown(context.Background())

	mgr, err := FromApp(app)
	if err != nil || mgr.Default() == nil {
		t.Fatalf("FromApp: %v", err)
	}
	ok, err := mgr.Allow(context.Background(), "test-key")
	if err != nil || !ok {
		t.Fatalf("Allow: ok=%v err=%v", ok, err)
	}
}

func TestLoadConfigFromEnvDefaults(t *testing.T) {
	for _, k := range []string{EnvDefault, EnvLimiters, EnvRate, EnvBurst, EnvPrefix} {
		t.Setenv(k, "")
	}
	cfg := LoadConfigFromEnv()
	if cfg.Default != rdriver.DriverMemory {
		t.Fatalf("default=%q", cfg.Default)
	}
	if len(cfg.Limiters) != 1 {
		t.Fatalf("limiters=%v", cfg.Limiters)
	}
}

func TestModuleDuplicateInitRejected(t *testing.T) {
	cfg := Config{
		Default: rdriver.DriverMemory,
		Limiters: map[string]LimiterConfig{
			"memory": {Driver: rdriver.DriverMemory, Spec: rdriver.Spec{Name: rdriver.DriverMemory}},
		},
	}
	app := framework.New("svc", "0.0.0").
		Use(ModuleWithConfig(cfg)).
		Use(ModuleWithConfig(cfg))
	if err := app.Init(context.Background()); err == nil {
		t.Fatalf("expected duplicate init error")
	}
}

func TestModuleContextHelpers(t *testing.T) {
	mgr := NewManager()
	ctx := ContextWithManager(context.Background(), mgr)
	got, ok := FromContext(ctx)
	if !ok || got != mgr {
		t.Fatalf("round trip failed")
	}
}

func TestFromAppMissingModule(t *testing.T) {
	app := framework.New("svc", "0.0.0")
	if err := app.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer app.Shutdown(context.Background())
	if _, err := FromApp(app); err == nil {
		t.Fatalf("expected error")
	}
}
