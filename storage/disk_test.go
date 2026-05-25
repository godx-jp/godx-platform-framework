package storage_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sort"
	"testing"

	"github.com/godx-jp/godx-platform-framework/framework"
	"github.com/godx-jp/godx-platform-framework/storage"
	"github.com/godx-jp/godx-platform-framework/storage/driver"
)

// newAppWithMemoryDisk builds a one-disk App backed by the memory
// driver, returns the matching Disk handle. Cleans up automatically.
func newAppWithMemoryDisk(t *testing.T) (*framework.App, *storage.Disk) {
	t.Helper()
	ctx := context.Background()
	cfg := storage.Config{
		DefaultDisk: "mem",
		Disks: map[string]storage.DiskConfig{
			"mem": {Driver: driver.DriverMemory},
		},
	}
	app := framework.New("test", "1.0.0").Use(storage.ModuleWithConfig(cfg))
	if err := app.Init(ctx); err != nil {
		t.Fatalf("init: %v", err)
	}
	t.Cleanup(func() { _ = app.Shutdown(context.Background()) })
	mgr, ok := storage.FromApp(app)
	if !ok {
		t.Fatal("FromApp returned no manager")
	}
	d, ok := mgr.Disk("mem")
	if !ok {
		t.Fatal("mem disk missing")
	}
	return app, d
}

func TestDisk_PutGet(t *testing.T) {
	_, d := newAppWithMemoryDisk(t)
	ctx := context.Background()

	if err := d.Put(ctx, "hello.txt", []byte("world")); err != nil {
		t.Fatalf("put: %v", err)
	}
	got, err := d.Get(ctx, "hello.txt")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if string(got) != "world" {
		t.Fatalf("get: want world, got %q", got)
	}
}

func TestDisk_PutWithOptions(t *testing.T) {
	_, d := newAppWithMemoryDisk(t)
	ctx := context.Background()

	if err := d.Put(ctx, "img.jpg", []byte("data"),
		storage.WithContentType("image/jpeg"),
		storage.WithVisibility(driver.VisibilityPublic),
		storage.WithMetadata(map[string]string{"author": "alice"}),
	); err != nil {
		t.Fatalf("put: %v", err)
	}
	attr, err := d.Attributes(ctx, "img.jpg")
	if err != nil {
		t.Fatalf("attrs: %v", err)
	}
	if attr.ContentType != "image/jpeg" {
		t.Fatalf("ct: %q", attr.ContentType)
	}
	if attr.Visibility != driver.VisibilityPublic {
		t.Fatalf("vis: %s", attr.Visibility)
	}
	if attr.Metadata["author"] != "alice" {
		t.Fatalf("metadata: %v", attr.Metadata)
	}
}

func TestDisk_MissingExistsDelete(t *testing.T) {
	_, d := newAppWithMemoryDisk(t)
	ctx := context.Background()

	missing, _ := d.Missing(ctx, "nope")
	if !missing {
		t.Fatal("want missing=true for unknown")
	}
	if err := d.Delete(ctx, "nope"); !errors.Is(err, driver.ErrNotFound) {
		t.Fatalf("delete missing: want ErrNotFound, got %v", err)
	}
}

func TestDisk_CopyAndMove(t *testing.T) {
	_, d := newAppWithMemoryDisk(t)
	ctx := context.Background()
	_ = d.Put(ctx, "src.txt", []byte("payload"))

	if err := d.Copy(ctx, "src.txt", "dst.txt"); err != nil {
		t.Fatalf("copy: %v", err)
	}
	got, _ := d.Get(ctx, "dst.txt")
	if string(got) != "payload" {
		t.Fatalf("copy dst: %q", got)
	}
	if ok, _ := d.Exists(ctx, "src.txt"); !ok {
		t.Fatal("src must still exist after copy")
	}

	if err := d.Move(ctx, "src.txt", "moved.txt"); err != nil {
		t.Fatalf("move: %v", err)
	}
	if ok, _ := d.Exists(ctx, "src.txt"); ok {
		t.Fatal("src must be gone after move")
	}
	if got, err := d.Get(ctx, "moved.txt"); err != nil || string(got) != "payload" {
		t.Fatalf("moved: got=%q err=%v", got, err)
	}
}

func TestDisk_FilesAndDirectories(t *testing.T) {
	_, d := newAppWithMemoryDisk(t)
	ctx := context.Background()
	for _, k := range []string{"a.txt", "b.txt", "sub/c.txt", "sub/d/e.txt"} {
		_ = d.Put(ctx, k, []byte("x"))
	}

	files, _ := d.Files(ctx, "")
	sort.Strings(files)
	if len(files) != 2 || files[0] != "a.txt" || files[1] != "b.txt" {
		t.Fatalf("files: %v", files)
	}

	dirs, _ := d.Directories(ctx, "")
	if len(dirs) != 1 || dirs[0] != "sub" {
		t.Fatalf("dirs: %v", dirs)
	}
}

func TestDisk_SizeAndLastModified(t *testing.T) {
	_, d := newAppWithMemoryDisk(t)
	ctx := context.Background()
	_ = d.Put(ctx, "k", []byte("hello"))

	sz, err := d.Size(ctx, "k")
	if err != nil || sz != 5 {
		t.Fatalf("size: %d err=%v", sz, err)
	}
	lm, err := d.LastModified(ctx, "k")
	if err != nil {
		t.Fatalf("lm: %v", err)
	}
	if lm.IsZero() {
		t.Fatal("expected non-zero last-modified")
	}
}

func TestDisk_StreamReadWrite(t *testing.T) {
	_, d := newAppWithMemoryDisk(t)
	ctx := context.Background()

	if err := d.WriteStream(ctx, "big.bin", bytes.NewReader([]byte("streamed"))); err != nil {
		t.Fatalf("writestream: %v", err)
	}
	r, err := d.ReadStream(ctx, "big.bin")
	if err != nil {
		t.Fatalf("readstream: %v", err)
	}
	defer r.Close()
	got, _ := io.ReadAll(r)
	if string(got) != "streamed" {
		t.Fatalf("readstream: %q", got)
	}
}

func TestDisk_AppendAndPrepend(t *testing.T) {
	_, d := newAppWithMemoryDisk(t)
	ctx := context.Background()

	if err := d.Append(ctx, "log.txt", []byte("hello\n")); err != nil {
		t.Fatalf("append1: %v", err)
	}
	if err := d.Append(ctx, "log.txt", []byte("world\n")); err != nil {
		t.Fatalf("append2: %v", err)
	}
	if err := d.Prepend(ctx, "log.txt", []byte("HEADER\n")); err != nil {
		t.Fatalf("prepend: %v", err)
	}
	got, _ := d.Get(ctx, "log.txt")
	if string(got) != "HEADER\nhello\nworld\n" {
		t.Fatalf("append/prepend: %q", got)
	}
}

func TestDisk_URLAndTemporaryURLOnMemoryReturnNotSupported(t *testing.T) {
	_, d := newAppWithMemoryDisk(t)
	if _, err := d.URL("x"); !errors.Is(err, driver.ErrNotSupported) {
		t.Fatalf("URL: %v", err)
	}
	if _, err := d.TemporaryURL(context.Background(), "x", 0); !errors.Is(err, driver.ErrNotSupported) {
		t.Fatalf("TemporaryURL: %v", err)
	}
	if _, err := d.TemporaryUploadURL(context.Background(), "x", 0); !errors.Is(err, driver.ErrNotSupported) {
		t.Fatalf("TemporaryUploadURL: %v", err)
	}
}

func TestDisk_DeletePrefix(t *testing.T) {
	_, d := newAppWithMemoryDisk(t)
	ctx := context.Background()
	for _, k := range []string{"a/1.txt", "a/b/2.txt", "c.txt"} {
		if err := d.Put(ctx, k, []byte("x")); err != nil {
			t.Fatalf("put %q: %v", k, err)
		}
	}
	if err := d.DeletePrefix(ctx, "a/"); err != nil {
		t.Fatalf("delete prefix: %v", err)
	}
	if ok, _ := d.Exists(ctx, "a/1.txt"); ok {
		t.Fatal("a/1.txt should be deleted")
	}
	if ok, _ := d.Exists(ctx, "a/b/2.txt"); ok {
		t.Fatal("a/b/2.txt should be deleted")
	}
	if ok, _ := d.Exists(ctx, "c.txt"); !ok {
		t.Fatal("c.txt should remain")
	}
}
