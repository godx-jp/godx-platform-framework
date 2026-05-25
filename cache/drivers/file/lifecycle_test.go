package file_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	cdriver "github.com/godx-jp/godx-platform-framework/cache/driver"
	_ "github.com/godx-jp/godx-platform-framework/cache/drivers/file"
)

func TestFile_ShutdownIsIdempotent(t *testing.T) {
	d, _ := newDriver(t)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if err := d.Shutdown(ctx); err != nil {
			t.Fatalf("shutdown %d: %v", i+1, err)
		}
	}
}

func TestFile_CorruptEnvelopeIsTreatedAsMiss(t *testing.T) {
	d, dir := newDriver(t)
	ctx := context.Background()
	_ = d.Put(ctx, "k", []byte("v"), 0)

	// Walk the root and overwrite the freshly-written .cache file
	// with garbage to simulate disk corruption / a partial write
	// from a crashed process.
	var path string
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, _ error) error {
		if info != nil && !info.IsDir() && filepath.Ext(p) == ".cache" {
			path = p
		}
		return nil
	})
	if path == "" {
		t.Fatal("no .cache file produced")
	}
	if err := os.WriteFile(path, []byte("garbage{not-json"), 0o644); err != nil {
		t.Fatalf("write garbage: %v", err)
	}

	// Get must surface the error and remove the file so the next Put
	// succeeds cleanly.
	if _, ok, err := d.Get(ctx, "k"); err == nil || ok {
		t.Fatalf("corrupt envelope: ok=%v err=%v (want non-nil error)", ok, err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("corrupt file was not removed: stat err = %v", err)
	}

	// Next Put repopulates cleanly.
	if err := d.Put(ctx, "k", []byte("fresh"), 0); err != nil {
		t.Fatalf("put after corruption: %v", err)
	}
	if v, ok, _ := d.Get(ctx, "k"); !ok || string(v) != "fresh" {
		t.Fatalf("recovery read: ok=%v v=%q", ok, v)
	}
}

func TestFile_TemporaryWritesAreCleanedUp(t *testing.T) {
	d, dir := newDriver(t)
	ctx := context.Background()
	for i := 0; i < 25; i++ {
		_ = d.Put(ctx, "k", []byte("v"), 0)
	}
	// No leftover .cache-* tmp files allowed.
	count := 0
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, _ error) error {
		if info == nil || info.IsDir() {
			return nil
		}
		name := filepath.Base(p)
		if len(name) >= 7 && name[:7] == ".cache-" {
			count++
		}
		return nil
	})
	if count != 0 {
		t.Fatalf("temporary write artefacts left behind: %d", count)
	}
}

func TestFile_ConcurrentWritersDontCorrupt(t *testing.T) {
	d, _ := newDriver(t)
	ctx := context.Background()
	const writers = 32
	const each = 50
	var wg sync.WaitGroup
	wg.Add(writers)
	for i := 0; i < writers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < each; j++ {
				if err := d.Put(ctx, "shared", []byte("v"), 0); err != nil {
					t.Errorf("put: %v", err)
					return
				}
				if v, ok, err := d.Get(ctx, "shared"); err != nil || !ok || string(v) != "v" {
					t.Errorf("get under contention: v=%q ok=%v err=%v", v, ok, err)
					return
				}
			}
		}()
	}
	wg.Wait()
}

func TestFile_FlushOnNonexistentRootIsNoop(t *testing.T) {
	dir := t.TempDir()
	d, err := cdriver.New(context.Background(), cdriver.Spec{Name: cdriver.DriverFile, Path: dir})
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	t.Cleanup(func() { _ = d.Shutdown(context.Background()) })

	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("rm dir: %v", err)
	}
	if err := d.Flush(context.Background()); err != nil {
		t.Fatalf("flush after rmdir: %v (should be a noop)", err)
	}
}

func TestFile_TTLPreservedAcrossIncrement(t *testing.T) {
	d, _ := newDriver(t)
	ctx := context.Background()
	_ = d.Put(ctx, "ctr", []byte("0"), 80*time.Millisecond)
	for i := 0; i < 3; i++ {
		if _, err := d.Increment(ctx, "ctr", 1); err != nil {
			t.Fatalf("incr: %v", err)
		}
	}
	// Value must still be visible before TTL elapses…
	time.Sleep(20 * time.Millisecond)
	if v, ok, _ := d.Get(ctx, "ctr"); !ok || string(v) != "3" {
		t.Fatalf("mid-TTL: v=%q ok=%v", v, ok)
	}
	// …and gone after TTL.
	time.Sleep(120 * time.Millisecond)
	if _, ok, _ := d.Get(ctx, "ctr"); ok {
		t.Fatal("counter outlived its TTL after Increment (TTL not preserved)")
	}
}
