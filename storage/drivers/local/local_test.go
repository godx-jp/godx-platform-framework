package local_test

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	stordriver "github.com/godx-jp/godx-platform-framework/storage/driver"
	"github.com/godx-jp/godx-platform-framework/storage/drivers/local"
)

func newDriver(t *testing.T, root string) stordriver.Driver {
	t.Helper()
	ctx := context.Background()
	d, err := stordriver.New(ctx, stordriver.Spec{
		Name: local.Name,
		Disk: "test",
		Root: root,
	})
	if err != nil {
		t.Fatalf("new local driver: %v", err)
	}
	t.Cleanup(func() { _ = d.Shutdown(ctx) })
	return d
}

func TestLocal_PutGetExistsDelete(t *testing.T) {
	d := newDriver(t, t.TempDir())
	ctx := context.Background()

	w, err := d.NewWriter(ctx, "hello.txt", stordriver.WriteOptions{})
	if err != nil {
		t.Fatalf("new writer: %v", err)
	}
	if _, err := w.Write([]byte("world")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	ok, err := d.Exists(ctx, "hello.txt")
	if err != nil || !ok {
		t.Fatalf("exists: ok=%v err=%v", ok, err)
	}

	r, err := d.NewReader(ctx, "hello.txt")
	if err != nil {
		t.Fatalf("reader: %v", err)
	}
	got, _ := io.ReadAll(r)
	_ = r.Close()
	if string(got) != "world" {
		t.Fatalf("get: want %q got %q", "world", got)
	}

	if err := d.Delete(ctx, "hello.txt"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := d.NewReader(ctx, "hello.txt"); !errors.Is(err, stordriver.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestLocal_PathTraversalRejected(t *testing.T) {
	d := newDriver(t, t.TempDir())
	ctx := context.Background()

	cases := []string{
		"../escape.txt",
		"foo/../../escape.txt",
		"",
		"/",
	}
	for _, key := range cases {
		t.Run(key, func(t *testing.T) {
			_, err := d.NewWriter(ctx, key, stordriver.WriteOptions{})
			if err == nil {
				t.Fatalf("expected error for key %q, got nil", key)
			}
		})
	}
}

// TestLocal_SymlinkEscapeRefused plants a symlink inside the root that
// points at a file outside the root and asserts that reads, writes and
// deletes through that symlink are refused by os.Root rather than
// touching the out-of-root target.
func TestLocal_SymlinkEscapeRefused(t *testing.T) {
	outside := t.TempDir()
	root := t.TempDir()

	// Secret living outside the storage root.
	secretPath := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secretPath, []byte("top-secret"), 0o600); err != nil {
		t.Fatalf("write secret: %v", err)
	}

	// Plant a symlink inside the root pointing at the out-of-root secret,
	// simulating an extracted archive or a hostile co-process.
	linkInRoot := filepath.Join(root, "evil.txt")
	if err := os.Symlink(secretPath, linkInRoot); err != nil {
		t.Skipf("symlinks unsupported on this platform: %v", err)
	}

	d := newDriver(t, root)
	ctx := context.Background()

	// Read through the symlink must be refused (no secret leakage).
	if r, err := d.NewReader(ctx, "evil.txt"); err == nil {
		_ = r.Close()
		t.Fatalf("NewReader followed symlink out of root; expected refusal")
	}

	// Delete through the symlink must be refused and must NOT remove the
	// out-of-root target.
	_ = d.Delete(ctx, "evil.txt")
	if _, err := os.Stat(secretPath); err != nil {
		t.Fatalf("out-of-root secret was deleted via symlink: %v", err)
	}

	// Write through the symlink must not clobber the out-of-root target.
	if w, err := d.NewWriter(ctx, "evil.txt", stordriver.WriteOptions{}); err == nil {
		_, _ = w.Write([]byte("overwritten"))
		_ = w.Close()
	}
	got, err := os.ReadFile(secretPath)
	if err != nil {
		t.Fatalf("read secret after write attempt: %v", err)
	}
	if string(got) != "top-secret" {
		t.Fatalf("out-of-root secret was overwritten via symlink: %q", got)
	}
}

func TestLocal_AutoCreatesParentDirs(t *testing.T) {
	root := t.TempDir()
	d := newDriver(t, root)
	ctx := context.Background()

	w, err := d.NewWriter(ctx, "a/b/c/file.bin", stordriver.WriteOptions{})
	if err != nil {
		t.Fatalf("writer: %v", err)
	}
	if _, err := w.Write([]byte("x")); err != nil {
		t.Fatalf("write: %v", err)
	}
	_ = w.Close()

	if ok, _ := d.Exists(ctx, "a/b/c/file.bin"); !ok {
		t.Fatalf("expected nested file present")
	}
	if !strings.HasPrefix(filepath.Join(root, "a"), root) {
		t.Fatalf("unexpected root")
	}
}

func TestLocal_VisibilityFromWriteOptions(t *testing.T) {
	d := newDriver(t, t.TempDir())
	ctx := context.Background()

	w, _ := d.NewWriter(ctx, "pub.txt", stordriver.WriteOptions{Visibility: stordriver.VisibilityPublic})
	_, _ = w.Write([]byte("hello"))
	_ = w.Close()

	attr, err := d.Attributes(ctx, "pub.txt")
	if err != nil {
		t.Fatalf("attrs: %v", err)
	}
	if attr.Visibility != stordriver.VisibilityPublic {
		t.Fatalf("want public, got %s", attr.Visibility)
	}

	w, _ = d.NewWriter(ctx, "priv.txt", stordriver.WriteOptions{Visibility: stordriver.VisibilityPrivate})
	_, _ = w.Write([]byte("secret"))
	_ = w.Close()
	attr, _ = d.Attributes(ctx, "priv.txt")
	if attr.Visibility != stordriver.VisibilityPrivate {
		t.Fatalf("want private, got %s", attr.Visibility)
	}
}

func TestLocal_ListReturnsImmediateChildrenOnly(t *testing.T) {
	d := newDriver(t, t.TempDir())
	ctx := context.Background()

	for _, k := range []string{"a.txt", "sub/b.txt", "sub/deep/c.txt", "sub/d.txt"} {
		w, _ := d.NewWriter(ctx, k, stordriver.WriteOptions{})
		_, _ = w.Write([]byte("x"))
		_ = w.Close()
	}

	got, err := d.List(ctx, "")
	if err != nil {
		t.Fatalf("list root: %v", err)
	}
	gotKeys := map[string]bool{}
	for _, e := range got {
		gotKeys[e.Key] = e.IsDir
	}
	if !gotKeys["a.txt"] || gotKeys["a.txt"] {
		// a.txt should be present and IsDir=false; isDir map's `gotKeys["a.txt"]` returns false on absent
	}
	if _, ok := gotKeys["a.txt"]; !ok {
		t.Fatalf("expected a.txt in root listing, got %v", gotKeys)
	}
	if isDir, ok := gotKeys["sub"]; !ok || !isDir {
		t.Fatalf("expected sub directory in root listing, got %v", gotKeys)
	}

	gotSub, _ := d.List(ctx, "sub")
	gotKeys = map[string]bool{}
	for _, e := range gotSub {
		gotKeys[e.Key] = e.IsDir
	}
	if _, ok := gotKeys["sub/b.txt"]; !ok {
		t.Fatalf("expected sub/b.txt in sub listing, got %v", gotKeys)
	}
	if isDir, ok := gotKeys["sub/deep"]; !ok || !isDir {
		t.Fatalf("expected sub/deep directory in sub listing, got %v", gotKeys)
	}
}

func TestLocal_URLRequiresPublicURL(t *testing.T) {
	d := newDriver(t, t.TempDir())
	if _, err := d.URL("any.txt"); !errors.Is(err, stordriver.ErrNotSupported) {
		t.Fatalf("expected ErrNotSupported without PublicURL, got %v", err)
	}

	ctx := context.Background()
	d2, err := stordriver.New(ctx, stordriver.Spec{
		Name:      local.Name,
		Disk:      "cdn",
		Root:      t.TempDir(),
		PublicURL: "https://cdn.example.com",
	})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	t.Cleanup(func() { _ = d2.Shutdown(ctx) })
	u, err := d2.URL("avatars/1.jpg")
	if err != nil {
		t.Fatalf("url: %v", err)
	}
	if want := "https://cdn.example.com/avatars/1.jpg"; u != want {
		t.Fatalf("URL: want %q got %q", want, u)
	}
}

func TestLocal_SignedURLNotSupported(t *testing.T) {
	d := newDriver(t, t.TempDir())
	if _, err := d.SignedURL(context.Background(), "x", 0); !errors.Is(err, stordriver.ErrNotSupported) {
		t.Fatalf("expected ErrNotSupported, got %v", err)
	}
}
