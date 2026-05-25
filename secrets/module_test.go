package secrets

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/godx-jp/godx-platform-framework/framework"
	sdriver "github.com/godx-jp/godx-platform-framework/secrets/driver"
)

func TestModuleWiresIntoApp(t *testing.T) {
	t.Setenv("SECRETS_HELLO", "world")
	cfg := Config{
		Default: "primary",
		Stores: map[string]StoreConfig{
			"primary": {
				Driver: sdriver.DriverEnv,
				Spec:   sdriver.Spec{Name: sdriver.DriverEnv},
			},
		},
	}
	app := framework.New("svc", "0.0.0").Use(ModuleWithConfig(cfg))
	ctx := context.Background()
	if err := app.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer func() { _ = app.Shutdown(ctx) }()

	mgr, err := FromApp(app)
	if err != nil {
		t.Fatalf("FromApp: %v", err)
	}
	v, err := mgr.GetString(ctx, "hello")
	if err != nil {
		t.Fatalf("GetString: %v", err)
	}
	if v != "world" {
		t.Fatalf("got %q", v)
	}
}

func TestModuleEnvBuildsFileStore(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/token", []byte("abc"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	t.Setenv(EnvDefault, sdriver.DriverFile)
	t.Setenv(EnvStores, sdriver.DriverFile)
	t.Setenv(EnvFilePath, dir)

	app := framework.New("svc", "0.0.0").Use(Module)
	ctx := context.Background()
	if err := app.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer func() { _ = app.Shutdown(ctx) }()

	mgr, err := FromApp(app)
	if err != nil {
		t.Fatalf("FromApp: %v", err)
	}
	v, err := mgr.GetString(ctx, "token")
	if err != nil {
		t.Fatalf("GetString: %v", err)
	}
	if v != "abc" {
		t.Fatalf("got %q", v)
	}
}

func TestLoadConfigFromEnvDefaults(t *testing.T) {
	for _, k := range []string{EnvDefault, EnvStores, EnvPrefix, EnvEnvPrefix, EnvFilePath, EnvVaultAddr, EnvVaultToken} {
		t.Setenv(k, "")
	}
	cfg := LoadConfigFromEnv()
	if cfg.Default != sdriver.DriverEnv {
		t.Fatalf("default=%q", cfg.Default)
	}
	if len(cfg.Stores) != 1 {
		t.Fatalf("stores=%v", cfg.Stores)
	}
}

func TestLoadConfigFromEnvMultiple(t *testing.T) {
	t.Setenv(EnvDefault, sdriver.DriverFile)
	t.Setenv(EnvStores, "env,file")
	t.Setenv(EnvFilePath, "/tmp/x")
	t.Setenv(EnvEnvPrefix, "APP_")
	cfg := LoadConfigFromEnv()
	if cfg.Default != sdriver.DriverFile {
		t.Fatalf("default=%q", cfg.Default)
	}
	if len(cfg.Stores) != 2 {
		t.Fatalf("stores=%v", cfg.Stores)
	}
	if cfg.Stores[sdriver.DriverFile].Spec.Path != "/tmp/x" {
		t.Fatalf("file path=%q", cfg.Stores[sdriver.DriverFile].Spec.Path)
	}
	if cfg.Stores[sdriver.DriverEnv].Spec.Prefix != "APP_" {
		t.Fatalf("env prefix=%q", cfg.Stores[sdriver.DriverEnv].Spec.Prefix)
	}
}

func TestValidateRejects(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  Config
	}{
		{"empty default", Config{Stores: map[string]StoreConfig{"x": {Driver: "env"}}}},
		{"no stores", Config{Default: "x"}},
		{"default not present", Config{Default: "x", Stores: map[string]StoreConfig{"y": {Driver: "env"}}}},
		{"driver missing", Config{Default: "x", Stores: map[string]StoreConfig{"x": {Driver: ""}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.cfg.Validate(); err == nil {
				t.Fatalf("expected error")
			}
		})
	}
}

func TestValidateAccepts(t *testing.T) {
	cfg := Config{
		Default: "x",
		Stores:  map[string]StoreConfig{"x": {Driver: "env", Spec: sdriver.Spec{Name: "env"}}},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestModuleDuplicateInitRejected(t *testing.T) {
	cfg := Config{
		Default: "x",
		Stores:  map[string]StoreConfig{"x": {Driver: sdriver.DriverEnv, Spec: sdriver.Spec{Name: sdriver.DriverEnv}}},
	}
	app := framework.New("svc", "0.0.0").
		Use(ModuleWithConfig(cfg)).
		Use(ModuleWithConfig(cfg))
	err := app.Init(context.Background())
	if err == nil {
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
	if _, ok := FromContext(context.Background()); ok {
		t.Fatalf("plain ctx should miss")
	}
	if _, ok := FromContext(nil); ok {
		t.Fatalf("nil ctx should miss")
	}
}

func TestFromAppMissingModule(t *testing.T) {
	app := framework.New("svc", "0.0.0")
	if err := app.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer func() { _ = app.Shutdown(context.Background()) }()
	if _, err := FromApp(app); err == nil {
		t.Fatalf("expected missing-module error")
	}
}

func TestModuleBubblesDriverConstructError(t *testing.T) {
	cfg := Config{
		Default: "file",
		Stores: map[string]StoreConfig{
			"file": {Driver: sdriver.DriverFile, Spec: sdriver.Spec{Name: sdriver.DriverFile /* Path missing */}},
		},
	}
	app := framework.New("svc", "0.0.0").Use(ModuleWithConfig(cfg))
	err := app.Init(context.Background())
	if err == nil {
		t.Fatalf("expected error for missing file path")
	}
}

// Ensure conformance with sentinel errors.
func TestSentinelExposed(t *testing.T) {
	if !errors.Is(sdriver.ErrNotFound, sdriver.ErrNotFound) {
		t.Fatal("sentinel identity broken")
	}
}
