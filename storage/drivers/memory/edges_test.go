package memory_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"

	stordriver "github.com/godx-jp/godx-platform-framework/storage/driver"
	"github.com/godx-jp/godx-platform-framework/storage/drivers/memory"
)

func TestMemory_WriteAfterCloseErrors(t *testing.T) {
	d := newDriver(t)
	ctx := context.Background()
	w, err := d.NewWriter(ctx, "k", stordriver.WriteOptions{})
	if err != nil {
		t.Fatalf("writer: %v", err)
	}
	_, _ = w.Write([]byte("x"))
	_ = w.Close()
	if _, err := w.Write([]byte("y")); err == nil {
		t.Fatal("write after close: want error")
	}
	// Double close is a no-op.
	if err := w.Close(); err != nil {
		t.Fatalf("double close: %v", err)
	}
}

func TestMemory_EmptyKeyRejected(t *testing.T) {
	d := newDriver(t)
	ctx := context.Background()
	for _, k := range []string{"", "   ", "/", `\`} {
		_, err := d.NewWriter(ctx, k, stordriver.WriteOptions{})
		if err == nil {
			t.Fatalf("empty/root key %q: want error", k)
		}
		_, err = d.NewReader(ctx, k)
		if err == nil {
			t.Fatalf("empty/root key %q reader: want error", k)
		}
	}
}

func TestMemory_MetadataAndContentTypePreserved(t *testing.T) {
	d := newDriver(t)
	ctx := context.Background()
	w, _ := d.NewWriter(ctx, "k", stordriver.WriteOptions{
		ContentType:  "application/json",
		Metadata:     map[string]string{"author": "alice", "purpose": "test"},
		CacheControl: "no-cache",
		Visibility:   stordriver.VisibilityPublic,
	})
	_, _ = w.Write([]byte(`{"x":1}`))
	_ = w.Close()

	attr, err := d.Attributes(ctx, "k")
	if err != nil {
		t.Fatalf("attrs: %v", err)
	}
	if attr.ContentType != "application/json" {
		t.Fatalf("content type: %q", attr.ContentType)
	}
	if attr.Visibility != stordriver.VisibilityPublic {
		t.Fatalf("visibility: %s", attr.Visibility)
	}
	if attr.Metadata["author"] != "alice" || attr.Metadata["purpose"] != "test" {
		t.Fatalf("metadata: %+v", attr.Metadata)
	}
	// Mutating the returned metadata map must NOT affect the cache.
	attr.Metadata["author"] = "mallory"
	attr2, _ := d.Attributes(ctx, "k")
	if attr2.Metadata["author"] != "alice" {
		t.Fatalf("metadata leaked internal map; second read author = %q", attr2.Metadata["author"])
	}
}

func TestMemory_ConcurrentWritersDifferentKeys(t *testing.T) {
	d := newDriver(t)
	ctx := context.Background()
	const goroutines = 32
	const each = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			for j := 0; j < each; j++ {
				key := stringFromN(i) + "-" + stringFromN(j)
				w, err := d.NewWriter(ctx, key, stordriver.WriteOptions{})
				if err != nil {
					t.Errorf("writer: %v", err)
					return
				}
				_, _ = w.Write([]byte("v"))
				_ = w.Close()
			}
		}()
	}
	wg.Wait()

	// Spot-check a handful and verify their existence.
	for i := 0; i < goroutines; i += 8 {
		for j := 0; j < each; j += 25 {
			key := stringFromN(i) + "-" + stringFromN(j)
			if ok, _ := d.Exists(ctx, key); !ok {
				t.Fatalf("missing key %s", key)
			}
		}
	}
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

func TestMemory_DefaultVisibilityFromSpec(t *testing.T) {
	d, err := stordriver.New(context.Background(), stordriver.Spec{
		Name:              memory.Name,
		DefaultVisibility: stordriver.VisibilityPublic,
	})
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	t.Cleanup(func() { _ = d.Shutdown(context.Background()) })
	w, _ := d.NewWriter(context.Background(), "k", stordriver.WriteOptions{})
	_, _ = w.Write([]byte("x"))
	_ = w.Close()
	attr, _ := d.Attributes(context.Background(), "k")
	if attr.Visibility != stordriver.VisibilityPublic {
		t.Fatalf("default visibility from Spec ignored: %s", attr.Visibility)
	}
}

func TestMemory_OverwriteReplacesAllFields(t *testing.T) {
	d := newDriver(t)
	ctx := context.Background()

	w, _ := d.NewWriter(ctx, "k", stordriver.WriteOptions{
		ContentType: "text/plain",
		Metadata:    map[string]string{"v": "1"},
	})
	_, _ = w.Write([]byte("first"))
	_ = w.Close()

	w, _ = d.NewWriter(ctx, "k", stordriver.WriteOptions{
		ContentType: "application/json",
		Metadata:    map[string]string{"v": "2"},
	})
	_, _ = w.Write([]byte(`{"a":1}`))
	_ = w.Close()

	attr, _ := d.Attributes(ctx, "k")
	if attr.ContentType != "application/json" {
		t.Fatalf("content-type not overwritten: %q", attr.ContentType)
	}
	if attr.Metadata["v"] != "2" {
		t.Fatalf("metadata not overwritten: %+v", attr.Metadata)
	}
	r, _ := d.NewReader(ctx, "k")
	body, _ := io.ReadAll(r)
	_ = r.Close()
	if string(body) != `{"a":1}` {
		t.Fatalf("body not overwritten: %q", body)
	}
}

func TestMemory_DeleteThenAttributesErrNotFound(t *testing.T) {
	d := newDriver(t)
	ctx := context.Background()
	w, _ := d.NewWriter(ctx, "k", stordriver.WriteOptions{})
	_, _ = w.Write([]byte("x"))
	_ = w.Close()
	_ = d.Delete(ctx, "k")
	if _, err := d.Attributes(ctx, "k"); !errors.Is(err, stordriver.ErrNotFound) {
		t.Fatalf("attrs after delete: want ErrNotFound, got %v", err)
	}
}

func TestMemory_PathTraversalNormalisedNotRejected(t *testing.T) {
	// memory driver has no concept of escaping a root, but its
	// cleanKey collapses traversal segments so they don't accidentally
	// create stray keys.
	d := newDriver(t)
	ctx := context.Background()
	w, err := d.NewWriter(ctx, "a/../b", stordriver.WriteOptions{})
	if err != nil {
		t.Fatalf("writer: %v", err)
	}
	_, _ = w.Write([]byte("v"))
	_ = w.Close()
	// Should land at key "b", not under any directory.
	if ok, _ := d.Exists(ctx, "b"); !ok {
		t.Fatal("expected key 'b' after a/../b normalisation")
	}
	keys, _ := d.List(ctx, "")
	for _, e := range keys {
		if strings.Contains(e.Key, "..") {
			t.Fatalf("traversal segment leaked into key: %q", e.Key)
		}
	}
}

func TestMemory_ShutdownIsIdempotent(t *testing.T) {
	d := newDriver(t)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if err := d.Shutdown(ctx); err != nil {
			t.Fatalf("shutdown %d: %v", i+1, err)
		}
	}
}
