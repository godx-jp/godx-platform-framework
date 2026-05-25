package file

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	cdriver "github.com/godx-jp/godx-platform-framework/config/driver"
)

func TestLoadYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.yaml")
	must(t, os.WriteFile(path, []byte(`
app:
  name: tiximax
  port: 8080
  features:
    - alpha
    - beta
`), 0o644))

	src, err := New(path, "", false)
	must(t, err)
	defer func() { _ = src.Shutdown(context.Background()) }()

	data, err := src.Load(context.Background())
	must(t, err)
	app, ok := data["app"].(map[string]any)
	if !ok {
		t.Fatalf("app should be map, got %#v", data["app"])
	}
	if app["name"] != "tiximax" {
		t.Fatalf("name unexpected: %v", app["name"])
	}
	flags, ok := app["features"].([]any)
	if !ok || len(flags) != 2 {
		t.Fatalf("features slice unexpected: %#v", app["features"])
	}
}

func TestLoadJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.json")
	must(t, os.WriteFile(path, []byte(`{"app":{"name":"tx","port":8080}}`), 0o644))

	src, err := New(path, "", false)
	must(t, err)
	defer func() { _ = src.Shutdown(context.Background()) }()

	data, err := src.Load(context.Background())
	must(t, err)
	if data["app"].(map[string]any)["name"] != "tx" {
		t.Fatalf("json round trip")
	}
}

func TestLoadTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.toml")
	must(t, os.WriteFile(path, []byte(`
[app]
name = "tx"
port = 8080
features = ["a", "b"]
`), 0o644))

	src, err := New(path, "", false)
	must(t, err)
	defer func() { _ = src.Shutdown(context.Background()) }()

	data, err := src.Load(context.Background())
	must(t, err)
	if data["app"].(map[string]any)["name"] != "tx" {
		t.Fatalf("toml round trip")
	}
}

func TestMissingPathOptional(t *testing.T) {
	src, err := New(filepath.Join(t.TempDir(), "missing.yaml"), "", true)
	must(t, err)
	defer func() { _ = src.Shutdown(context.Background()) }()

	data, err := src.Load(context.Background())
	must(t, err)
	if len(data) != 0 {
		t.Fatalf("missing optional file should yield empty tree")
	}
}

func TestMissingPathRequired(t *testing.T) {
	src, err := New(filepath.Join(t.TempDir(), "missing.yaml"), "", false)
	must(t, err)
	defer func() { _ = src.Shutdown(context.Background()) }()

	_, err = src.Load(context.Background())
	if !errors.Is(err, cdriver.ErrFileMissing) {
		t.Fatalf("expected ErrFileMissing, got %v", err)
	}
}

func TestUnsupportedFormatExt(t *testing.T) {
	_, err := New("cfg.xml", "", false)
	if !errors.Is(err, cdriver.ErrUnsupportedFormat) {
		t.Fatalf("expected ErrUnsupportedFormat, got %v", err)
	}
}

func TestExplicitFormatOverridesExt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "noext")
	must(t, os.WriteFile(path, []byte(`{"a":1}`), 0o644))
	src, err := New(path, "json", false)
	must(t, err)
	defer func() { _ = src.Shutdown(context.Background()) }()

	data, err := src.Load(context.Background())
	must(t, err)
	if data["a"] == nil {
		t.Fatalf("explicit json format failed")
	}
}

func TestShutdownIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.yaml")
	must(t, os.WriteFile(path, []byte("a: 1"), 0o644))
	src, err := New(path, "", false)
	must(t, err)
	if err := src.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown 1: %v", err)
	}
	if err := src.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown 2 should be safe: %v", err)
	}
	if _, err := src.Load(context.Background()); !errors.Is(err, cdriver.ErrClosed) {
		t.Fatalf("expected ErrClosed after shutdown, got %v", err)
	}
}

func TestWatchFiresOnChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.yaml")
	must(t, os.WriteFile(path, []byte("a: 1"), 0o644))
	src, err := New(path, "", false)
	must(t, err)
	defer func() { _ = src.Shutdown(context.Background()) }()

	w, ok := src.(cdriver.Watcher)
	if !ok {
		t.Fatalf("file source should implement Watcher")
	}
	ch := make(chan struct{}, 4)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := w.Watch(ctx, func() { ch <- struct{}{} }); err != nil {
		t.Fatalf("Watch: %v", err)
	}
	time.Sleep(150 * time.Millisecond)
	// bump mtime by writing a new value at least 2s later (poll period is 1s; ext4 mtime granularity is 1s).
	time.Sleep(1500 * time.Millisecond)
	must(t, os.WriteFile(path, []byte("a: 2"), 0o644))
	select {
	case <-ch:
	case <-ctx.Done():
		t.Fatalf("watch did not fire on change")
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
