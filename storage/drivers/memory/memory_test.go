package memory_test

import (
	"context"
	"errors"
	"io"
	"sort"
	"testing"

	stordriver "github.com/godx-jp/godx-platform-framework/storage/driver"
	"github.com/godx-jp/godx-platform-framework/storage/drivers/memory"
)

func newDriver(t *testing.T) stordriver.Driver {
	t.Helper()
	ctx := context.Background()
	d, err := stordriver.New(ctx, stordriver.Spec{Name: memory.Name, Disk: "test"})
	if err != nil {
		t.Fatalf("new memory: %v", err)
	}
	t.Cleanup(func() { _ = d.Shutdown(ctx) })
	return d
}

func TestMemory_RoundTrip(t *testing.T) {
	d := newDriver(t)
	ctx := context.Background()

	w, err := d.NewWriter(ctx, "k1", stordriver.WriteOptions{ContentType: "text/plain"})
	if err != nil {
		t.Fatalf("writer: %v", err)
	}
	_, _ = w.Write([]byte("hello"))
	_ = w.Close()

	ok, _ := d.Exists(ctx, "k1")
	if !ok {
		t.Fatal("expected exists")
	}

	r, _ := d.NewReader(ctx, "k1")
	got, _ := io.ReadAll(r)
	_ = r.Close()
	if string(got) != "hello" {
		t.Fatalf("get: want hello got %q", got)
	}

	attr, err := d.Attributes(ctx, "k1")
	if err != nil {
		t.Fatalf("attrs: %v", err)
	}
	if attr.Size != 5 || attr.ContentType != "text/plain" {
		t.Fatalf("attrs unexpected: %+v", attr)
	}
}

func TestMemory_DeleteNotFound(t *testing.T) {
	d := newDriver(t)
	if err := d.Delete(context.Background(), "nope"); !errors.Is(err, stordriver.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestMemory_ListGroupsByDirectory(t *testing.T) {
	d := newDriver(t)
	ctx := context.Background()

	keys := []string{"a.txt", "sub/b.txt", "sub/deep/c.txt", "sub/d.txt"}
	for _, k := range keys {
		w, _ := d.NewWriter(ctx, k, stordriver.WriteOptions{})
		_, _ = w.Write([]byte("x"))
		_ = w.Close()
	}

	root, err := d.List(ctx, "")
	if err != nil {
		t.Fatalf("list root: %v", err)
	}
	var files, dirs []string
	for _, e := range root {
		if e.IsDir {
			dirs = append(dirs, e.Key)
		} else {
			files = append(files, e.Key)
		}
	}
	sort.Strings(files)
	sort.Strings(dirs)
	if len(files) != 1 || files[0] != "a.txt" {
		t.Fatalf("root files = %v, want [a.txt]", files)
	}
	if len(dirs) != 1 || dirs[0] != "sub" {
		t.Fatalf("root dirs = %v, want [sub]", dirs)
	}

	sub, _ := d.List(ctx, "sub")
	files, dirs = nil, nil
	for _, e := range sub {
		if e.IsDir {
			dirs = append(dirs, e.Key)
		} else {
			files = append(files, e.Key)
		}
	}
	sort.Strings(files)
	sort.Strings(dirs)
	if len(files) != 2 || files[0] != "sub/b.txt" || files[1] != "sub/d.txt" {
		t.Fatalf("sub files = %v", files)
	}
	if len(dirs) != 1 || dirs[0] != "sub/deep" {
		t.Fatalf("sub dirs = %v", dirs)
	}
}

func TestMemory_URLAndSignedURLNotSupported(t *testing.T) {
	d := newDriver(t)
	if _, err := d.URL("k"); !errors.Is(err, stordriver.ErrNotSupported) {
		t.Fatalf("URL want ErrNotSupported, got %v", err)
	}
	if _, err := d.SignedURL(context.Background(), "k", 0); !errors.Is(err, stordriver.ErrNotSupported) {
		t.Fatalf("SignedURL want ErrNotSupported, got %v", err)
	}
}

func TestMemory_ShutdownClearsObjects(t *testing.T) {
	d := newDriver(t)
	ctx := context.Background()

	w, _ := d.NewWriter(ctx, "k", stordriver.WriteOptions{})
	_, _ = w.Write([]byte("x"))
	_ = w.Close()

	if err := d.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	if ok, _ := d.Exists(ctx, "k"); ok {
		t.Fatal("expected objects cleared after shutdown")
	}
}
