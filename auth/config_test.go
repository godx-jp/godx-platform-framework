package auth

import (
	"testing"

	adriver "github.com/godx-jp/godx-platform-framework/auth/driver"
)

func TestLoadConfigFromEnvMultiGuard(t *testing.T) {
	t.Setenv(EnvDefault, "apikey")
	t.Setenv(EnvGuards, "jwt,apikey")
	t.Setenv("AUTH_GUARD_JWT_DRIVER", "jwt")
	t.Setenv("AUTH_GUARD_JWT_JWKS_URL", "https://idp.example.com/jwks")
	t.Setenv("AUTH_GUARD_JWT_ISSUER", "https://idp.example.com")
	t.Setenv("AUTH_GUARD_APIKEY_DRIVER", "apikey")
	t.Setenv("AUTH_GUARD_APIKEY_KEYS", "svc:secret123,bot:botkey")
	t.Setenv("AUTH_GUARD_APIKEY_HEADER", "X-Custom-Key")

	cfg := LoadConfigFromEnv()
	if cfg.Default != "apikey" {
		t.Fatalf("Default=%q", cfg.Default)
	}
	jwt := cfg.Guards["jwt"]
	if jwt.Driver != adriver.DriverJWT || jwt.Spec.JWKSURL == "" || jwt.Spec.Issuer == "" {
		t.Fatalf("jwt spec=%+v", jwt)
	}
	api := cfg.Guards["apikey"]
	if api.Driver != adriver.DriverAPIKey || api.Spec.Header != "X-Custom-Key" {
		t.Fatalf("apikey spec=%+v", api)
	}
	if len(api.Spec.Keys) != 2 {
		t.Fatalf("keys=%v", api.Spec.Keys)
	}
	if api.Spec.Keys["svc"].Secret != "secret123" || api.Spec.Keys["bot"].Secret != "botkey" {
		t.Fatalf("parsed keys=%v", api.Spec.Keys)
	}
}

func TestConfigValidateRejectsUnknownDefault(t *testing.T) {
	cfg := Config{
		Default: "missing",
		Guards: map[string]GuardConfig{
			"jwt": {Driver: adriver.DriverJWT},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestConfigValidateRejectsEmptyDriver(t *testing.T) {
	cfg := Config{
		Default: "jwt",
		Guards: map[string]GuardConfig{
			"jwt": {Driver: ""},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestParseAPIKeysSkipsMalformed(t *testing.T) {
	keys := parseAPIKeys("apikey", "good:secret, bad, :empty, ok:val")
	if len(keys) != 2 {
		t.Fatalf("keys=%v", keys)
	}
	if keys["good"].Secret != "secret" || keys["ok"].Secret != "val" {
		t.Fatalf("keys=%v", keys)
	}
}
