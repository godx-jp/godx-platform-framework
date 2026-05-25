package config

import (
	"context"
	"errors"
	"testing"

	cdriver "github.com/godx-jp/godx-platform-framework/config/driver"
	"github.com/godx-jp/godx-platform-framework/config/drivers/static"
)

func TestManagerMergeOrder(t *testing.T) {
	a := static.New(map[string]any{
		"app": map[string]any{"name": "from-a", "x": 1},
	})
	b := static.New(map[string]any{
		"app": map[string]any{"name": "from-b", "y": 2},
	})
	mgr := NewManager()
	defer func() { _ = mgr.Shutdown(context.Background()) }()
	must(t, mgr.AddSource(context.Background(), "first", a))
	must(t, mgr.AddSource(context.Background(), "second", b))

	repo := mgr.Repository()
	if repo.GetString("app.name", "") != "from-b" {
		t.Fatalf("second source should override first: got %q", repo.GetString("app.name", ""))
	}
	if repo.GetInt("app.x", -1) != 1 {
		t.Fatalf("first-source-only key lost")
	}
	if repo.GetInt("app.y", -1) != 2 {
		t.Fatalf("second-source-only key missing")
	}
}

func TestManagerDuplicateSource(t *testing.T) {
	mgr := NewManager()
	defer func() { _ = mgr.Shutdown(context.Background()) }()
	must(t, mgr.AddSource(context.Background(), "a", static.New(nil)))
	if err := mgr.AddSource(context.Background(), "a", static.New(nil)); err == nil {
		t.Fatalf("duplicate name should be rejected")
	}
}

func TestManagerNilSource(t *testing.T) {
	mgr := NewManager()
	if err := mgr.AddSource(context.Background(), "x", nil); err == nil {
		t.Fatalf("nil source should be rejected")
	}
}

func TestManagerReload(t *testing.T) {
	s := static.New(map[string]any{"k": "v1"}).(interface{ Update(map[string]any) })
	mgr := NewManager()
	defer func() { _ = mgr.Shutdown(context.Background()) }()
	must(t, mgr.AddSource(context.Background(), "s", s.(cdriver.Source)))

	if mgr.Repository().GetString("k", "") != "v1" {
		t.Fatalf("initial load")
	}
	s.Update(map[string]any{"k": "v2"})
	must(t, mgr.Reload(context.Background()))
	if mgr.Repository().GetString("k", "") != "v2" {
		t.Fatalf("reload did not pick up update")
	}
}

func TestManagerOnChange(t *testing.T) {
	mgr := NewManager()
	defer func() { _ = mgr.Shutdown(context.Background()) }()
	fired := 0
	mgr.OnChange(func(r *Repository) { fired++ })
	must(t, mgr.AddSource(context.Background(), "s", static.New(map[string]any{"a": 1})))
	if fired == 0 {
		t.Fatalf("OnChange should fire on initial AddSource reload")
	}
	must(t, mgr.Reload(context.Background()))
	if fired < 2 {
		t.Fatalf("OnChange should fire on subsequent reload")
	}
}

func TestManagerShutdownIdempotent(t *testing.T) {
	mgr := NewManager()
	must(t, mgr.AddSource(context.Background(), "s", static.New(nil)))
	if err := mgr.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown 1: %v", err)
	}
	if err := mgr.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown 2 should be safe")
	}
	if err := mgr.AddSource(context.Background(), "x", static.New(nil)); !errors.Is(err, cdriver.ErrClosed) {
		t.Fatalf("expected ErrClosed, got %v", err)
	}
	if err := mgr.Reload(context.Background()); !errors.Is(err, cdriver.ErrClosed) {
		t.Fatalf("expected ErrClosed on reload, got %v", err)
	}
}

func TestManagerSourcesList(t *testing.T) {
	mgr := NewManager()
	defer func() { _ = mgr.Shutdown(context.Background()) }()
	must(t, mgr.AddSource(context.Background(), "a", static.New(nil)))
	must(t, mgr.AddSource(context.Background(), "b", static.New(nil)))
	got := mgr.Sources()
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("Sources unexpected: %v", got)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
