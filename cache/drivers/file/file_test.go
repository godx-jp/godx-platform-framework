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

func newDriver(t *testing.T) (cdriver.Driver, string) {
	t.Helper()
	dir := t.TempDir()
	d, err := cdriver.New(context.Background(), cdriver.Spec{
		Name: cdriver.DriverFile,
		Path: dir,
	})
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	t.Cleanup(func() { _ = d.Shutdown(context.Background()) })
	return d, dir
}

func TestFile_PathRequired(t *testing.T) {
	_, err := cdriver.New(context.Background(), cdriver.Spec{Name: cdriver.DriverFile})
	if err == nil {
		t.Fatal("want error when path is empty")
	}
}

func TestFile_PutGetForget(t *testing.T) {
	d, _ := newDriver(t)
	ctx := context.Background()
	if err := d.Put(ctx, "k", []byte("hello"), 0); err != nil {
		t.Fatalf("put: %v", err)
	}
	v, ok, err := d.Get(ctx, "k")
	if err != nil || !ok || string(v) != "hello" {
		t.Fatalf("get: v=%q ok=%v err=%v", v, ok, err)
	}
	_ = d.Forget(ctx, "k")
	if _, ok, _ := d.Get(ctx, "k"); ok {
		t.Fatal("get after forget: still present")
	}
}

func TestFile_TTL(t *testing.T) {
	d, _ := newDriver(t)
	ctx := context.Background()
	_ = d.Put(ctx, "k", []byte("v"), 10*time.Millisecond)
	time.Sleep(25 * time.Millisecond)
	if _, ok, _ := d.Get(ctx, "k"); ok {
		t.Fatal("expected expired key to be invisible")
	}
}

func TestFile_AddIsAtomic(t *testing.T) {
	d, _ := newDriver(t)
	ctx := context.Background()
	const n = 100
	var wg sync.WaitGroup
	wg.Add(n)
	wins := make(chan bool, n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			ok, _ := d.Add(ctx, "race", []byte("v"), 0)
			wins <- ok
		}()
	}
	wg.Wait()
	close(wins)
	count := 0
	for v := range wins {
		if v {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("Add must succeed exactly once under contention; got %d", count)
	}
}

func TestFile_IncrementPersistsAcrossInstances(t *testing.T) {
	dir := t.TempDir()
	mk := func() cdriver.Driver {
		d, err := cdriver.New(context.Background(), cdriver.Spec{Name: cdriver.DriverFile, Path: dir})
		if err != nil {
			t.Fatalf("construct: %v", err)
		}
		return d
	}
	ctx := context.Background()
	d1 := mk()
	n, err := d1.Increment(ctx, "c", 7)
	if err != nil || n != 7 {
		t.Fatalf("d1 incr: n=%d err=%v", n, err)
	}
	_ = d1.Shutdown(ctx)

	d2 := mk()
	defer d2.Shutdown(ctx)
	n, err = d2.Increment(ctx, "c", 3)
	if err != nil || n != 10 {
		t.Fatalf("d2 incr persisted: n=%d err=%v", n, err)
	}
	_, err = d2.Decrement(ctx, "c", 4)
	if err != nil {
		t.Fatalf("d2 decr: %v", err)
	}
}

func TestFile_IncrementRejectsNonInteger(t *testing.T) {
	d, _ := newDriver(t)
	ctx := context.Background()
	_ = d.Put(ctx, "bad", []byte("hello"), 0)
	if _, err := d.Increment(ctx, "bad", 1); !errors.Is(err, cdriver.ErrNotInteger) {
		t.Fatalf("want ErrNotInteger, got %v", err)
	}
}

func TestFile_Flush(t *testing.T) {
	d, dir := newDriver(t)
	ctx := context.Background()
	for _, k := range []string{"a", "b", "c"} {
		_ = d.Put(ctx, k, []byte(k), 0)
	}
	if err := d.Flush(ctx); err != nil {
		t.Fatalf("flush: %v", err)
	}
	for _, k := range []string{"a", "b", "c"} {
		if _, ok, _ := d.Get(ctx, k); ok {
			t.Fatalf("flush should have removed %q", k)
		}
	}
	// No *.cache files should remain.
	leftover := 0
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, _ error) error {
		if info != nil && !info.IsDir() && filepath.Ext(p) == ".cache" {
			leftover++
		}
		return nil
	})
	if leftover != 0 {
		t.Fatalf("flush left %d .cache files behind", leftover)
	}
}

func TestFile_OnDiskLayoutShardsByHash(t *testing.T) {
	d, dir := newDriver(t)
	ctx := context.Background()
	_ = d.Put(ctx, "hello", []byte("world"), 0)
	// Expect at least one file under <dir>/XX/YY/*.cache
	var found bool
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, _ error) error {
		if info != nil && !info.IsDir() && filepath.Ext(p) == ".cache" {
			rel, _ := filepath.Rel(dir, p)
			if len(rel) >= 5 && rel[2] == filepath.Separator && rel[5] == filepath.Separator {
				found = true
			}
		}
		return nil
	})
	if !found {
		t.Fatal("expected hashed-shard directory layout (XX/YY/<hash>.cache)")
	}
}
