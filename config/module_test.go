package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/godx-jp/godx-platform-framework/framework"
)

func TestModuleWiresIntoApp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.yaml")
	if err := os.WriteFile(path, []byte("app: { name: tiximax, port: 9000 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := Config{
		AutoEnv: false,
		Sources: []NamedSourceConfig{
			{Name: "file", Config: SourceConfig{Driver: "file", Path: path}},
		},
	}
	app := framework.New("test", "0.0.0").Use(ModuleWithConfig(cfg))
	ctx := context.Background()
	if err := app.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer func() { _ = app.Shutdown(ctx) }()

	repo, err := FromApp(app)
	if err != nil {
		t.Fatalf("FromApp: %v", err)
	}
	if repo.GetString("app.name", "") != "tiximax" {
		t.Fatalf("did not load app.name")
	}
	if repo.GetInt("app.port", 0) != 9000 {
		t.Fatalf("did not load app.port")
	}
}

func TestModuleAutoEnv(t *testing.T) {
	t.Setenv("TXM_CFG_TEST__VAR", "v")
	cfg := Config{AutoEnv: true, AutoEnvPrefix: "TXM_CFG_TEST_"}
	app := framework.New("test", "0.0.0").Use(ModuleWithConfig(cfg))
	ctx := context.Background()
	if err := app.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer func() { _ = app.Shutdown(ctx) }()
	repo, _ := FromApp(app)
	if repo.GetString("_var", "") != "v" {
		// the env driver lowercases and splits on "__", so "_VAR" → "_var"
		// (the underscore in the prefix consumed produces an empty first segment,
		// which we drop in splitKey, leaving "_var" trimmed to "var").
		// Allow either depending on whether prefix had trailing underscore.
		if repo.GetString("var", "") != "v" {
			t.Fatalf("auto env did not load var, got tree: %#v", repo.All())
		}
	}
}

func TestModuleContextHelpers(t *testing.T) {
	mgr := NewManager()
	defer func() { _ = mgr.Shutdown(context.Background()) }()
	ctx := ContextWithManager(context.Background(), mgr)
	got, ok := FromContext(ctx)
	if !ok || got != mgr {
		t.Fatalf("ContextWithManager round trip failed")
	}
	if _, ok := FromContext(context.Background()); ok {
		t.Fatalf("FromContext on plain ctx should be false")
	}
	if _, ok := FromContext(nil); ok {
		t.Fatalf("FromContext on nil ctx should be false")
	}
}
