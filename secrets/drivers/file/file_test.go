package file

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"testing"

	sdriver "github.com/godx-jp/godx-platform-framework/secrets/driver"
)

func TestRegisteredOnImport(t *testing.T) {
	if sdriver.Lookup(sdriver.DriverFile) == nil {
		t.Fatalf("file driver not registered")
	}
}

func newStore(t *testing.T) sdriver.Store {
	t.Helper()
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = s.Shutdown(context.Background()) })
	return s
}

func TestPutGetRoundtrip(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	if err := s.Put(ctx, "db/password", []byte("hunter2")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	v, err := s.Get(ctx, "db/password")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(v) != "hunter2" {
		t.Fatalf("v=%q", v)
	}
}

func TestGetMissing(t *testing.T) {
	s := newStore(t)
	_, err := s.Get(context.Background(), "nope")
	if !errors.Is(err, sdriver.ErrNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestGetEmptyKey(t *testing.T) {
	s := newStore(t)
	_, err := s.Get(context.Background(), "")
	if !errors.Is(err, sdriver.ErrNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestTrailingNewlineTrimmed(t *testing.T) {
	dir := t.TempDir()
	s, _ := New(dir)
	defer s.Shutdown(context.Background())
	if err := os.WriteFile(filepath.Join(dir, "token"), []byte("abc\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	v, err := s.Get(context.Background(), "token")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(v) != "abc" {
		t.Fatalf("v=%q (newline not stripped)", v)
	}
}

func TestK8sStyleMount(t *testing.T) {
	// K8s mounts secrets as files with a base-name-only path. Make
	// sure we read them with a flat key.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "username"), []byte("alice"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	s, _ := New(dir)
	defer s.Shutdown(context.Background())
	v, err := s.Get(context.Background(), "username")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(v) != "alice" {
		t.Fatalf("v=%q", v)
	}
}

func TestForgetMissingIsNoop(t *testing.T) {
	s := newStore(t)
	if err := s.Forget(context.Background(), "missing"); err != nil {
		t.Fatalf("Forget missing: %v", err)
	}
}

func TestForgetRemoves(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	_ = s.Put(ctx, "k", []byte("v"))
	if err := s.Forget(ctx, "k"); err != nil {
		t.Fatalf("Forget: %v", err)
	}
	_, err := s.Get(ctx, "k")
	if !errors.Is(err, sdriver.ErrNotFound) {
		t.Fatalf("after Forget err=%v", err)
	}
}

func TestPathTraversalRejected(t *testing.T) {
	s := newStore(t)
	for _, k := range []string{"../etc/passwd", "/etc/passwd", "sub/../../etc/passwd"} {
		if _, err := s.Get(context.Background(), k); err == nil {
			t.Fatalf("expected error for %q", k)
		}
		if err := s.Put(context.Background(), k, []byte("x")); err == nil {
			t.Fatalf("Put should reject %q", k)
		}
	}
}

func TestListReturnsRelativeKeys(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	_ = s.Put(ctx, "a", []byte("1"))
	_ = s.Put(ctx, "nested/b", []byte("2"))
	keys, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	sort.Strings(keys)
	if len(keys) != 2 || keys[0] != "a" || keys[1] != "nested/b" {
		t.Fatalf("keys=%v", keys)
	}
}

func TestListEmpty(t *testing.T) {
	s := newStore(t)
	keys, err := s.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(keys) != 0 {
		t.Fatalf("expected empty, got %v", keys)
	}
}

func TestPutCreatesParentDir(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	if err := s.Put(ctx, "very/deep/nested/key", []byte("x")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	v, err := s.Get(ctx, "very/deep/nested/key")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(v) != "x" {
		t.Fatalf("v=%q", v)
	}
}

func TestPutOverwrites(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	_ = s.Put(ctx, "k", []byte("old"))
	if err := s.Put(ctx, "k", []byte("new")); err != nil {
		t.Fatalf("overwrite: %v", err)
	}
	v, _ := s.Get(ctx, "k")
	if string(v) != "new" {
		t.Fatalf("v=%q", v)
	}
}

func TestPutAtomicMode(t *testing.T) {
	dir := t.TempDir()
	s, _ := New(dir)
	defer s.Shutdown(context.Background())
	_ = s.Put(context.Background(), "k", []byte("x"))
	info, err := os.Stat(filepath.Join(dir, "k"))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode=%v", info.Mode().Perm())
	}
}

func TestAfterShutdown(t *testing.T) {
	dir := t.TempDir()
	s, _ := New(dir)
	if err := s.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if _, err := s.Get(context.Background(), "k"); !errors.Is(err, sdriver.ErrClosed) {
		t.Fatalf("Get err=%v", err)
	}
	if err := s.Put(context.Background(), "k", []byte("v")); !errors.Is(err, sdriver.ErrClosed) {
		t.Fatalf("Put err=%v", err)
	}
}

func TestShutdownIdempotent(t *testing.T) {
	dir := t.TempDir()
	s, _ := New(dir)
	if err := s.Shutdown(context.Background()); err != nil {
		t.Fatalf("first: %v", err)
	}
	if err := s.Shutdown(context.Background()); err != nil {
		t.Fatalf("second: %v", err)
	}
}

func TestConstructorRequiresPath(t *testing.T) {
	if _, err := New(""); err == nil {
		t.Fatalf("empty root should error")
	}
	c := sdriver.Lookup(sdriver.DriverFile)
	if _, err := c(context.Background(), sdriver.Spec{Name: sdriver.DriverFile}); err == nil {
		t.Fatalf("missing spec.Path should error")
	}
}

func TestConstructorViaRegistryWithPrefix(t *testing.T) {
	root := t.TempDir()
	s, err := sdriver.New(context.Background(), sdriver.Spec{
		Name:   sdriver.DriverFile,
		Path:   root,
		Prefix: "tenant-a",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer s.Shutdown(context.Background())
	if err := s.Put(context.Background(), "k", []byte("v")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "tenant-a", "k")); err != nil {
		t.Fatalf("file at expected sub-root not present: %v", err)
	}
}
