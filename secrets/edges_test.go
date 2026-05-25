package secrets

import (
	"context"
	"sync"
	"testing"

	sdriver "github.com/godx-jp/godx-platform-framework/secrets/driver"
)

func TestManagerSafeForConcurrentReads(t *testing.T) {
	m := NewManager()
	for _, n := range []string{"a", "b", "c"} {
		_ = m.AddStore(n, newFake(n))
	}
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = m.Default()
			_, _ = m.Store("a")
			_ = m.Stores()
		}(i)
	}
	wg.Wait()
}

func TestManagerShutdownSafeIfNoStores(t *testing.T) {
	m := NewManager()
	if err := m.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
}

func TestManagerHandlesNilContext(t *testing.T) {
	m := NewManager()
	_ = m.AddStore("a", newFake("a"))
	// Drivers should not panic on nil context — Manager passes
	// through.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panic: %v", r)
		}
	}()
	//nolint:staticcheck // intentionally pass nil ctx.
	_, _ = m.Get(nil, "k")
}

func TestEmptySpecDriverRejected(t *testing.T) {
	if _, err := sdriver.New(context.Background(), sdriver.Spec{}); err == nil {
		t.Fatalf("expected error for empty spec")
	}
}
