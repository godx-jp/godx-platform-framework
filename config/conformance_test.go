package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	cdriver "github.com/godx-jp/godx-platform-framework/config/driver"
)

// TestConformanceSources runs the driver-agnostic behaviour suite
// across every registered Source driver that can be wired up from a
// minimal Spec without external infrastructure. Heavy drivers
// (remote KV) are exercised by their own integration test suites.
func TestConformanceSources(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "c.yaml")
	if err := os.WriteFile(yamlPath, []byte("a: 1\nnested:\n  x: 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name  string
		spec  cdriver.Spec
		setup func(t *testing.T)
	}{
		{name: "env", spec: cdriver.Spec{Name: cdriver.DriverEnv, Prefix: "CFGCONFORM_"}, setup: func(t *testing.T) {
			t.Setenv("CFGCONFORM_FOO", "bar")
		}},
		{name: "file/yaml", spec: cdriver.Spec{Name: cdriver.DriverFile, Path: yamlPath}, setup: func(*testing.T) {}},
		{name: "static", spec: cdriver.Spec{Name: cdriver.DriverStatic}, setup: func(*testing.T) {}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			tc.setup(t)
			src, err := cdriver.New(context.Background(), tc.spec)
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			defer func() { _ = src.Shutdown(context.Background()) }()

			data, err := src.Load(context.Background())
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			// Drivers may return empty trees (env without matches, static seeded empty).
			_ = data

			if src.Name() == "" {
				t.Fatalf("Name should not be empty")
			}

			if err := src.Shutdown(context.Background()); err != nil {
				t.Fatalf("Shutdown 1: %v", err)
			}
			if err := src.Shutdown(context.Background()); err != nil {
				t.Fatalf("Shutdown 2 should be safe: %v", err)
			}
		})
	}
}
