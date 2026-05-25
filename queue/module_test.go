package queue

import (
	"context"
	"testing"

	"github.com/godx-jp/godx-platform-framework/framework"
)

func TestModuleWiresManager(t *testing.T) {
	ctx := context.Background()
	app := framework.New("test", "0").Use(ModuleWithConfig(Config{
		Default: "default",
		Queues: map[string]QueueConfig{
			"default": {Driver: "memory", DefaultQueue: "jobs"},
		},
	}, nil))
	if err := app.Init(ctx); err != nil {
		t.Fatal(err)
	}
	mgr, err := FromApp(app)
	if err != nil {
		t.Fatal(err)
	}
	q, err := mgr.Default()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := q.Push(ctx, "jobs", []byte("ok")); err != nil {
		t.Fatal(err)
	}
}

func TestModuleDuplicateRejected(t *testing.T) {
	cfg := Config{
		Default: "default",
		Queues: map[string]QueueConfig{
			"default": {Driver: "memory"},
		},
	}
	app := framework.New("test", "0").Use(ModuleWithConfig(cfg, nil)).Use(ModuleWithConfig(cfg, nil))
	if err := app.Init(context.Background()); err == nil {
		t.Fatal("expected duplicate init error")
	}
}

func TestConfigValidate(t *testing.T) {
	if err := (Config{}).Validate(); err == nil {
		t.Fatal("expected error")
	}
}
