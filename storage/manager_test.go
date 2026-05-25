package storage_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/godx-jp/godx-platform-framework/framework"
	"github.com/godx-jp/godx-platform-framework/storage"
	"github.com/godx-jp/godx-platform-framework/storage/driver"
)

func TestManager_MultipleDisksAndDefault(t *testing.T) {
	ctx := context.Background()
	cfg := storage.Config{
		DefaultDisk: "mem",
		Disks: map[string]storage.DiskConfig{
			"mem":    {Driver: driver.DriverMemory},
			"local1": {Driver: driver.DriverLocal, Root: t.TempDir()},
		},
	}
	app := framework.New("svc", "1.0").Use(storage.ModuleWithConfig(cfg))
	if err := app.Init(ctx); err != nil {
		t.Fatalf("init: %v", err)
	}
	t.Cleanup(func() { _ = app.Shutdown(context.Background()) })

	mgr, _ := storage.FromApp(app)
	if got := mgr.Disks(); len(got) != 2 || got[0] != "local1" || got[1] != "mem" {
		t.Fatalf("Disks(): %v", got)
	}
	if name := mgr.DefaultName(); name != "mem" {
		t.Fatalf("default: %q", name)
	}
	if d, ok := mgr.Default(); !ok || d == nil {
		t.Fatal("Default() missing")
	}
	if _, ok := mgr.Disk("nope"); ok {
		t.Fatal("nope must not exist")
	}
}

func TestManager_RejectsUnknownDriver(t *testing.T) {
	app := framework.New("svc", "1.0").Use(storage.ModuleWithConfig(storage.Config{
		DefaultDisk: "x",
		Disks: map[string]storage.DiskConfig{
			"x": {Driver: "nonexistent", Root: "/tmp"},
		},
	}))
	err := app.Init(context.Background())
	if err == nil || !strings.Contains(err.Error(), "unknown driver") {
		t.Fatalf("want unknown-driver error, got %v", err)
	}
}

func TestManager_HeavyDriverHintsAtBlankImport(t *testing.T) {
	// s3 is registered as a stub by the s3 package, but we do NOT import
	// it here. So driver.New must return the unknown-driver hint.
	app := framework.New("svc", "1.0").Use(storage.ModuleWithConfig(storage.Config{
		DefaultDisk: "primary",
		Disks: map[string]storage.DiskConfig{
			"primary": {Driver: "s3", Bucket: "b"},
		},
	}))
	err := app.Init(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	// Either the stub is registered (returns ErrNotImplemented) or the
	// driver is unknown (returns blank-import hint). Both are correct
	// failure modes — verify the message is helpful in either case.
	msg := err.Error()
	if !strings.Contains(msg, "blank import") && !strings.Contains(msg, "not yet implemented") {
		t.Fatalf("error must hint blank-import OR not-implemented; got %v", err)
	}
}

func TestManager_AddDiskAfterModule(t *testing.T) {
	ctx := context.Background()
	app := framework.New("svc", "1.0").
		Use(storage.ModuleWithConfig(storage.Config{
			DefaultDisk: "mem",
			Disks: map[string]storage.DiskConfig{
				"mem": {Driver: driver.DriverMemory},
			},
		})).
		Use(storage.AddDisk("extra", storage.DiskConfig{Driver: driver.DriverMemory}))

	if err := app.Init(ctx); err != nil {
		t.Fatalf("init: %v", err)
	}
	t.Cleanup(func() { _ = app.Shutdown(context.Background()) })

	mgr, _ := storage.FromApp(app)
	if _, ok := mgr.Disk("extra"); !ok {
		t.Fatal("extra disk missing")
	}
}

func TestManager_AddDiskOrderingErrorWhenBeforeModule(t *testing.T) {
	app := framework.New("svc", "1.0").
		Use(storage.AddDisk("extra", storage.DiskConfig{Driver: driver.DriverMemory})).
		Use(storage.ModuleWithConfig(storage.Config{
			DefaultDisk: "mem",
			Disks: map[string]storage.DiskConfig{
				"mem": {Driver: driver.DriverMemory},
			},
		}))
	err := app.Init(context.Background())
	if err == nil || !strings.Contains(err.Error(), "must be wired before AddDisk") {
		t.Fatalf("want ordering error, got %v", err)
	}
}

func TestManager_FromAppEmptyWhenModuleNotWired(t *testing.T) {
	app := framework.New("svc", "1.0")
	if _, ok := storage.FromApp(app); ok {
		t.Fatal("FromApp must return false when Module is not wired")
	}
}

func TestManager_FromContextRoundtrip(t *testing.T) {
	mgr := storage.NewManager()
	ctx := storage.ContextWithManager(context.Background(), mgr)
	got, ok := storage.FromContext(ctx)
	if !ok || got != mgr {
		t.Fatalf("FromContext: ok=%v got=%v want=%v", ok, got, mgr)
	}
	if _, ok := storage.FromContext(context.Background()); ok {
		t.Fatal("FromContext must return false for bare context")
	}
}

func TestManager_RejectsSecondModuleInit(t *testing.T) {
	app := framework.New("svc", "1.0").
		Use(storage.ModuleWithConfig(storage.Config{
			DefaultDisk: "mem",
			Disks:       map[string]storage.DiskConfig{"mem": {Driver: driver.DriverMemory}},
		})).
		Use(storage.ModuleWithConfig(storage.Config{
			DefaultDisk: "mem2",
			Disks:       map[string]storage.DiskConfig{"mem2": {Driver: driver.DriverMemory}},
		}))
	err := app.Init(context.Background())
	if err == nil || !strings.Contains(err.Error(), "already initialised") {
		t.Fatalf("want already-initialised error, got %v", err)
	}
}

func TestManager_EnvDrivenMultipleDisks(t *testing.T) {
	t.Setenv(storage.EnvDefaultDisk, "uploads")
	t.Setenv(storage.EnvDisks, "uploads,cache")
	root := t.TempDir()
	t.Setenv("STORAGE_DISK_UPLOADS_DRIVER", "local")
	t.Setenv("STORAGE_DISK_UPLOADS_ROOT", root)
	t.Setenv("STORAGE_DISK_CACHE_DRIVER", "memory")

	app := framework.New("svc", "1.0").Use(storage.Module)
	if err := app.Init(context.Background()); err != nil {
		t.Fatalf("init: %v", err)
	}
	t.Cleanup(func() { _ = app.Shutdown(context.Background()) })

	mgr, _ := storage.FromApp(app)
	if mgr.DefaultName() != "uploads" {
		t.Fatalf("default = %q", mgr.DefaultName())
	}
	if _, ok := mgr.Disk("uploads"); !ok {
		t.Fatal("uploads disk missing")
	}
	if _, ok := mgr.Disk("cache"); !ok {
		t.Fatal("cache disk missing")
	}
}

func TestManager_MustDiskPanicsOnMiss(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic")
		}
		if s, ok := r.(string); !ok || !strings.Contains(s, "not registered") {
			t.Fatalf("panic message = %v", r)
		}
	}()
	mgr := storage.NewManager()
	_ = mgr.MustDisk("ghost")
}

func TestManager_BuildDiskNotFoundError(t *testing.T) {
	if _, err := driver.New(context.Background(), driver.Spec{Name: "ghost"}); err == nil {
		t.Fatal("expected error for ghost driver")
	} else if !strings.Contains(err.Error(), "blank import") {
		t.Fatalf("error must include blank-import hint, got %v", err)
	}
	// Sanity: the registry contains the light drivers we expect.
	names := driver.Names()
	want := []string{driver.DriverLocal, driver.DriverMemory}
	for _, w := range want {
		found := false
		for _, n := range names {
			if n == w {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("driver %q must be auto-registered (names=%v)", w, names)
		}
	}
}

func TestManager_ShutdownIsIdempotent(t *testing.T) {
	mgr := storage.NewManager()
	// Construct directly to avoid the app machinery.
	d, err := driver.New(context.Background(), driver.Spec{Name: driver.DriverMemory})
	if err != nil {
		t.Fatalf("new mem: %v", err)
	}
	_ = mgr.AddDisk("x", manualDisk("x", d))
	_ = mgr.SetDefault("x")
	if err := mgr.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown1: %v", err)
	}
	if err := mgr.Shutdown(context.Background()); err != nil && !errors.Is(err, nil) {
		t.Fatalf("shutdown2: %v", err)
	}
}

// manualDisk is a tiny test helper that wraps a driver into a Disk
// without going through buildDisk (which we already exercise above).
// We need an exported constructor; mirror the unexported one used by
// the module via storage.NewDiskFromDriver below.
func manualDisk(name string, d driver.Driver) *storage.Disk {
	return storage.NewDiskFromDriver(name, d, storage.DiskConfig{Driver: "memory"})
}
