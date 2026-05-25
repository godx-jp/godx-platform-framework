package auth

import (
	"context"
	"testing"

	"github.com/godx-jp/godx-platform-framework/framework"
	adriver "github.com/godx-jp/godx-platform-framework/auth/driver"
)

func TestModuleWires(t *testing.T) {
	cfg := Config{
		Default: "api",
		Guards: map[string]GuardConfig{
			"api": {
				Driver: adriver.DriverAPIKey,
				Spec: adriver.Spec{
					Name: adriver.DriverAPIKey,
					Keys: map[string]adriver.APIKeyEntry{
						"svc": {SubjectID: "svc", Secret: "secret"},
					},
				},
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
	p, err := mgr.Authenticate(context.Background(), &adriver.CredentialRequest{APIKey: "secret"})
	if err != nil || p.SubjectID != "svc" {
		t.Fatalf("Authenticate: p=%v err=%v", p, err)
	}
}

func TestLoadConfigFromEnvDefaults(t *testing.T) {
	for _, k := range []string{EnvDefault, EnvGuards} {
		t.Setenv(k, "")
	}
	cfg := LoadConfigFromEnv()
	if cfg.Default != adriver.DriverAPIKey {
		t.Fatalf("default=%q", cfg.Default)
	}
	if len(cfg.Guards) != 1 {
		t.Fatalf("guards=%v", cfg.Guards)
	}
}

func TestModuleDuplicateInitRejected(t *testing.T) {
	cfg := Config{
		Default: "api",
		Guards: map[string]GuardConfig{
			"api": {Driver: adriver.DriverAPIKey, Spec: adriver.Spec{Name: adriver.DriverAPIKey}},
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
		t.Fatalf("manager round trip failed")
	}
	p := &Principal{SubjectID: "u1"}
	ctx = ContextWithPrincipal(ctx, p)
	gotP, ok := PrincipalFromContext(ctx)
	if !ok || gotP.SubjectID != "u1" {
		t.Fatalf("principal round trip failed")
	}
	id, ok := UserIDFromContext(ctx)
	if !ok || id != "u1" {
		t.Fatalf("UserIDFromContext=%q ok=%v", id, ok)
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
