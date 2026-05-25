package featureflag

import (
	"context"
	"testing"

	"github.com/godx-jp/godx-platform-framework/config"
	"github.com/godx-jp/godx-platform-framework/framework"
)

func TestModuleWiresIntoApp(t *testing.T) {
	repo := config.NewRepository(map[string]any{
		"flags": map[string]any{"new-ui": true},
	})
	cfg := Config{Driver: "config", Prefix: "flags"}
	app := framework.New("svc", "0.0.0").Use(ModuleWithConfig(cfg, repo))
	ctx := context.Background()
	if err := app.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer func() { _ = app.Shutdown(ctx) }()

	eval, err := FromApp(app)
	if err != nil {
		t.Fatalf("FromApp: %v", err)
	}
	ok, err := eval.Enabled(ctx, "new-ui", "alice", nil)
	if err != nil || !ok {
		t.Fatalf("Enabled: ok=%v err=%v", ok, err)
	}
}

func TestModuleDuplicateInitRejected(t *testing.T) {
	repo := config.NewRepository(nil)
	app := framework.New("svc", "0.0.0").
		Use(ModuleWithConfig(Config{}, repo)).
		Use(ModuleWithConfig(Config{}, repo))
	if err := app.Init(context.Background()); err == nil {
		t.Fatalf("expected duplicate init error")
	}
}
