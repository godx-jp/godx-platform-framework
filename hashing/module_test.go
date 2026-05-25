package hashing

import (
	"context"
	"testing"

	hdriver "github.com/godx-jp/godx-platform-framework/hashing/driver"
	"github.com/godx-jp/godx-platform-framework/framework"
)

func TestModuleWiresIntoApp(t *testing.T) {
	cfg := Config{
		Default: "primary",
		Hashers: map[string]HasherConfig{
			"primary": {Driver: hdriver.DriverBcrypt, Spec: hdriver.Spec{Name: hdriver.DriverBcrypt, BcryptCost: 4}},
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
	if mgr.Default().Name() != hdriver.DriverBcrypt {
		t.Fatalf("default name wrong")
	}
}

func TestLoadConfigFromEnvDefaults(t *testing.T) {
	for _, k := range []string{EnvDefault, EnvHashers, EnvBcryptCost, EnvArgon2Time, EnvArgon2Memory, EnvArgon2Threads, EnvScryptN, EnvScryptR, EnvScryptP} {
		t.Setenv(k, "")
	}
	cfg := LoadConfigFromEnv()
	if cfg.Default != hdriver.DriverBcrypt || len(cfg.Hashers) != 1 {
		t.Fatalf("env defaults wrong: %+v", cfg)
	}
}

func TestLoadConfigFromEnvOverrides(t *testing.T) {
	t.Setenv(EnvDefault, hdriver.DriverArgon2id)
	t.Setenv(EnvHashers, "argon2id,bcrypt")
	t.Setenv(EnvBcryptCost, "8")
	t.Setenv(EnvArgon2Time, "4")
	t.Setenv(EnvArgon2Memory, "131072")
	t.Setenv(EnvArgon2Threads, "4")
	cfg := LoadConfigFromEnv()
	if cfg.Default != hdriver.DriverArgon2id {
		t.Fatalf("default wrong")
	}
	if cfg.Hashers[hdriver.DriverBcrypt].Spec.BcryptCost != 8 {
		t.Fatalf("bcrypt cost wrong: %+v", cfg.Hashers[hdriver.DriverBcrypt].Spec)
	}
	if cfg.Hashers[hdriver.DriverArgon2id].Spec.Argon2Time != 4 {
		t.Fatalf("argon2id time wrong: %+v", cfg.Hashers[hdriver.DriverArgon2id].Spec)
	}
}

func TestValidateRejects(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  Config
	}{
		{"empty default", Config{Hashers: map[string]HasherConfig{"x": {Driver: "bcrypt"}}}},
		{"no hashers", Config{Default: "x"}},
		{"default not present", Config{Default: "x", Hashers: map[string]HasherConfig{"y": {Driver: "bcrypt"}}}},
		{"driver missing", Config{Default: "x", Hashers: map[string]HasherConfig{"x": {Driver: ""}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.cfg.Validate(); err == nil {
				t.Fatalf("expected error")
			}
		})
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
