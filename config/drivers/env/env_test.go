package env

import (
	"context"
	"errors"
	"testing"

	cdriver "github.com/godx-jp/godx-platform-framework/config/driver"
)

func TestEnvPrefixedAndNested(t *testing.T) {
	t.Setenv("MYAPP_DB__HOST", "localhost")
	t.Setenv("MYAPP_DB__PORT", "5432")
	t.Setenv("MYAPP_NAME", "x")
	t.Setenv("OTHER_VAR", "ignored")

	src := New("MYAPP_", "__")
	data, err := src.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	db, ok := data["db"].(map[string]any)
	if !ok {
		t.Fatalf("db should be map, got %#v", data["db"])
	}
	if db["host"] != "localhost" || db["port"] != "5432" {
		t.Fatalf("db unexpected: %#v", db)
	}
	if data["name"] != "x" {
		t.Fatalf("name unexpected: %v", data["name"])
	}
	if _, ok := data["other_var"]; ok {
		t.Fatalf("OTHER_VAR should be filtered out by prefix")
	}
}

func TestEnvAfterShutdown(t *testing.T) {
	src := New("", "__")
	if err := src.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if _, err := src.Load(context.Background()); !errors.Is(err, cdriver.ErrClosed) {
		t.Fatalf("expected ErrClosed, got %v", err)
	}
}

func TestEnvRegistryAutoRegisters(t *testing.T) {
	c := cdriver.Lookup(cdriver.DriverEnv)
	if c == nil {
		t.Fatalf("env driver should auto-register")
	}
	src, err := c(context.Background(), cdriver.Spec{Name: cdriver.DriverEnv})
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	if src.Name() != cdriver.DriverEnv {
		t.Fatalf("Name unexpected: %q", src.Name())
	}
}
