package local_test

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	stordriver "github.com/godx-jp/godx-platform-framework/storage/driver"
	"github.com/godx-jp/godx-platform-framework/storage/drivers/local"
)

func TestLocal_Construct_EmptyRootRejected(t *testing.T) {
	_, err := stordriver.New(context.Background(), stordriver.Spec{Name: local.Name})
	if err == nil || !strings.Contains(err.Error(), "root path is required") {
		t.Fatalf("want root-required error, got %v", err)
	}
}

func TestLocal_AbsoluteKey_StaysUnderRoot(t *testing.T) {
	root := t.TempDir()
	d := newDriver(t, root)
	ctx := context.Background()

	// Absolute-looking keys must NOT escape the configured root.
	// path.Clean("/" + "/etc/passwd") => "/etc/passwd", joined under root.
	w, err := d.NewWriter(ctx, "/etc/passwd", stordriver.WriteOptions{})
	if err != nil {
		t.Fatalf("writer absolute key: %v", err)
	}
	if _, err := w.Write([]byte("not your /etc/passwd")); err != nil {
		t.Fatalf("write: %v", err)
	}
	_ = w.Close()

	// The file must exist under <root>/etc/passwd, not the system /etc/passwd.
	abs := filepath.Join(root, "etc", "passwd")
	if _, err := os.Stat(abs); err != nil {
		t.Fatalf("expected file under root at %s: %v", abs, err)
	}
	if _, err := os.Stat("/etc/passwd-godx-test"); err == nil {
		t.Fatal("driver wrote outside its root!")
	}
}

func TestLocal_BackslashesNormalised(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("backslash semantics differ on Windows")
	}
	d := newDriver(t, t.TempDir())
	ctx := context.Background()
	// "foo\\bar.txt" on Unix should be treated as the nested path
	// "foo/bar.txt" — driver normalises backslashes for parity with
	// Laravel's Storage facade on multi-OS deployments.
	w, err := d.NewWriter(ctx, `foo\bar.txt`, stordriver.WriteOptions{})
	if err != nil {
		t.Fatalf("writer: %v", err)
	}
	_, _ = w.Write([]byte("v"))
	_ = w.Close()
	if ok, _ := d.Exists(ctx, "foo/bar.txt"); !ok {
		t.Fatal("backslash key was not normalised to forward-slash path")
	}
}

func TestLocal_ExistsOnDirectoryReturnsFalse(t *testing.T) {
	root := t.TempDir()
	d := newDriver(t, root)
	ctx := context.Background()
	w, _ := d.NewWriter(ctx, "dir/inside.txt", stordriver.WriteOptions{})
	_, _ = w.Write([]byte("x"))
	_ = w.Close()
	if ok, err := d.Exists(ctx, "dir"); err != nil || ok {
		t.Fatalf("dir as object: ok=%v err=%v (Laravel parity: false)", ok, err)
	}
}

func TestLocal_ReadingDirectoryErrors(t *testing.T) {
	root := t.TempDir()
	d := newDriver(t, root)
	ctx := context.Background()
	if err := os.MkdirAll(filepath.Join(root, "subdir"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Opening a directory as an io.Reader either works (succeeds at
	// open but errors on Read) or errors immediately depending on the
	// OS. Either is acceptable as long as we never silently return
	// garbage to the caller.
	r, err := d.NewReader(ctx, "subdir")
	if err != nil {
		return // acceptable: open refused
	}
	defer r.Close()
	if _, err := io.ReadAll(r); err == nil {
		t.Fatal("ReadAll on a directory returned no error")
	}
}

func TestLocal_DeleteMissingReturnsErrNotFound(t *testing.T) {
	d := newDriver(t, t.TempDir())
	if err := d.Delete(context.Background(), "absent.bin"); !errors.Is(err, stordriver.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestLocal_AttributesMissingReturnsErrNotFound(t *testing.T) {
	d := newDriver(t, t.TempDir())
	if _, err := d.Attributes(context.Background(), "absent.bin"); !errors.Is(err, stordriver.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestLocal_ListEmptyRootIsClean(t *testing.T) {
	d := newDriver(t, t.TempDir())
	entries, err := d.List(context.Background(), "")
	if err != nil {
		t.Fatalf("list empty: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("empty root: %d entries (want 0)", len(entries))
	}
}

func TestLocal_ListMissingPrefixIsEmpty(t *testing.T) {
	d := newDriver(t, t.TempDir())
	entries, err := d.List(context.Background(), "no/such/dir")
	if err != nil {
		t.Fatalf("list missing: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("missing prefix: %d entries (want 0)", len(entries))
	}
}

func TestLocal_ListOnFileErrors(t *testing.T) {
	d := newDriver(t, t.TempDir())
	ctx := context.Background()
	w, _ := d.NewWriter(ctx, "regular.txt", stordriver.WriteOptions{})
	_, _ = w.Write([]byte("x"))
	_ = w.Close()
	if _, err := d.List(ctx, "regular.txt"); err == nil {
		t.Fatal("List on a regular file should error")
	}
}

func TestLocal_VisibilityDefaultFromSpec(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()
	d, err := stordriver.New(ctx, stordriver.Spec{
		Name:              local.Name,
		Root:              root,
		DefaultVisibility: stordriver.VisibilityPublic,
	})
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	t.Cleanup(func() { _ = d.Shutdown(ctx) })

	w, _ := d.NewWriter(ctx, "f.txt", stordriver.WriteOptions{}) // no per-write visibility
	_, _ = w.Write([]byte("x"))
	_ = w.Close()
	attr, _ := d.Attributes(ctx, "f.txt")
	if attr.Visibility != stordriver.VisibilityPublic {
		t.Fatalf("default visibility from Spec ignored: %s", attr.Visibility)
	}
}

func TestLocal_ShutdownIsIdempotent(t *testing.T) {
	d := newDriver(t, t.TempDir())
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if err := d.Shutdown(ctx); err != nil {
			t.Fatalf("shutdown %d: %v", i+1, err)
		}
	}
}

func TestLocal_ConcurrentWritesDifferentKeys(t *testing.T) {
	d := newDriver(t, t.TempDir())
	ctx := context.Background()
	const goroutines = 16
	const each = 25
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			for j := 0; j < each; j++ {
				key := strings.TrimSpace(stringFromN(i)) + "/" + stringFromN(j) + ".bin"
				w, err := d.NewWriter(ctx, key, stordriver.WriteOptions{})
				if err != nil {
					t.Errorf("writer: %v", err)
					return
				}
				if _, err := w.Write([]byte("x")); err != nil {
					t.Errorf("write: %v", err)
					return
				}
				_ = w.Close()
				if ok, _ := d.Exists(ctx, key); !ok {
					t.Errorf("just-written key missing: %s", key)
					return
				}
			}
		}()
	}
	wg.Wait()
}

func stringFromN(n int) string {
	if n == 0 {
		return "0"
	}
	var buf []byte
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	return string(buf)
}

func TestLocal_LargeFileRoundTrip(t *testing.T) {
	d := newDriver(t, t.TempDir())
	ctx := context.Background()
	const size = 4 << 20 // 4 MiB
	payload := make([]byte, size)
	for i := range payload {
		payload[i] = byte(i)
	}
	w, _ := d.NewWriter(ctx, "big.bin", stordriver.WriteOptions{})
	if _, err := w.Write(payload); err != nil {
		t.Fatalf("write: %v", err)
	}
	_ = w.Close()

	attr, err := d.Attributes(ctx, "big.bin")
	if err != nil {
		t.Fatalf("attr: %v", err)
	}
	if attr.Size != int64(size) {
		t.Fatalf("attr size = %d, want %d", attr.Size, size)
	}

	r, _ := d.NewReader(ctx, "big.bin")
	got, err := io.ReadAll(r)
	_ = r.Close()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(got) != size {
		t.Fatalf("read len = %d, want %d", len(got), size)
	}
	if string(got[:64]) != string(payload[:64]) || string(got[size-64:]) != string(payload[size-64:]) {
		t.Fatal("payload mismatch at boundaries")
	}
}

func TestLocal_URL_EncodesSpecialCharsButNotSlash(t *testing.T) {
	d, err := stordriver.New(context.Background(), stordriver.Spec{
		Name:      local.Name,
		Root:      t.TempDir(),
		PublicURL: "https://cdn.example.com/storage",
	})
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	t.Cleanup(func() { _ = d.Shutdown(context.Background()) })

	u, err := d.URL("users/42/avatar 1.png")
	if err != nil {
		t.Fatalf("url: %v", err)
	}
	want := "https://cdn.example.com/storage/users/42/avatar%201.png"
	if u != want {
		t.Fatalf("URL: want %q got %q", want, u)
	}
}
