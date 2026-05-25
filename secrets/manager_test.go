package secrets

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"

	sdriver "github.com/godx-jp/godx-platform-framework/secrets/driver"
)

// fakeStore is an in-memory Store used to exercise Manager logic
// without depending on a particular driver.
type fakeStore struct {
	name string
	mu   sync.Mutex
	data map[string][]byte
	down bool
	err  error
}

func newFake(name string) *fakeStore {
	return &fakeStore{name: name, data: map[string][]byte{}}
}

func (f *fakeStore) Name() string { return f.name }

func (f *fakeStore) Get(_ context.Context, k string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.data[k]
	if !ok {
		return nil, sdriver.ErrNotFound
	}
	return v, nil
}

func (f *fakeStore) Put(_ context.Context, k string, v []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.data[k] = append([]byte(nil), v...)
	return nil
}

func (f *fakeStore) Forget(_ context.Context, k string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.data, k)
	return nil
}

func (f *fakeStore) List(_ context.Context) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	keys := make([]string, 0, len(f.data))
	for k := range f.data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys, nil
}

func (f *fakeStore) Shutdown(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.down = true
	return f.err
}

func TestManagerAddDuplicateRejected(t *testing.T) {
	m := NewManager()
	if err := m.AddStore("a", newFake("a")); err != nil {
		t.Fatalf("first: %v", err)
	}
	if err := m.AddStore("a", newFake("a")); err == nil {
		t.Fatalf("expected duplicate error")
	}
}

func TestManagerAddNilRejected(t *testing.T) {
	m := NewManager()
	if err := m.AddStore("a", nil); err == nil {
		t.Fatalf("expected nil error")
	}
	if err := m.AddStore("", newFake("a")); err == nil {
		t.Fatalf("expected empty name error")
	}
}

func TestManagerFirstAddBecomesDefault(t *testing.T) {
	m := NewManager()
	a := newFake("a")
	b := newFake("b")
	_ = m.AddStore("a", a)
	_ = m.AddStore("b", b)
	if m.Default() != a {
		t.Fatalf("default not first added")
	}
}

func TestManagerSetDefaultUnknown(t *testing.T) {
	m := NewManager()
	if err := m.SetDefault("missing"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestManagerStoreUnknown(t *testing.T) {
	m := NewManager()
	if _, err := m.Store("missing"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestManagerNoDefaultErrors(t *testing.T) {
	m := NewManager()
	_, err := m.Get(context.Background(), "k")
	if err == nil {
		t.Fatalf("expected no-default error")
	}
	if err := m.Put(context.Background(), "k", []byte("v")); err == nil {
		t.Fatalf("Put expected no-default error")
	}
	if err := m.Forget(context.Background(), "k"); err == nil {
		t.Fatalf("Forget expected no-default error")
	}
}

func TestManagerStringHelpers(t *testing.T) {
	m := NewManager()
	_ = m.AddStore("a", newFake("a"))
	ctx := context.Background()
	if err := m.PutString(ctx, "k", "hello"); err != nil {
		t.Fatalf("PutString: %v", err)
	}
	v, err := m.GetString(ctx, "k")
	if err != nil {
		t.Fatalf("GetString: %v", err)
	}
	if v != "hello" {
		t.Fatalf("v=%q", v)
	}
}

func TestManagerGetStringPropagatesError(t *testing.T) {
	m := NewManager()
	_ = m.AddStore("a", newFake("a"))
	_, err := m.GetString(context.Background(), "missing")
	if !errors.Is(err, sdriver.ErrNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestManagerStoresSorted(t *testing.T) {
	m := NewManager()
	_ = m.AddStore("b", newFake("b"))
	_ = m.AddStore("a", newFake("a"))
	_ = m.AddStore("c", newFake("c"))
	got := m.Stores()
	if got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Fatalf("not sorted: %v", got)
	}
}

func TestManagerShutdownJoinsErrors(t *testing.T) {
	m := NewManager()
	a := newFake("a")
	b := newFake("b")
	b.err = errors.New("boom")
	_ = m.AddStore("a", a)
	_ = m.AddStore("b", b)
	err := m.Shutdown(context.Background())
	if err == nil || !errors.Is(err, b.err) {
		t.Fatalf("err=%v", err)
	}
	if !a.down || !b.down {
		t.Fatalf("not all shut down")
	}
}

func TestManagerSwitchDefault(t *testing.T) {
	m := NewManager()
	a := newFake("a")
	b := newFake("b")
	_ = m.AddStore("a", a)
	_ = m.AddStore("b", b)
	if err := m.SetDefault("b"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}
	if m.Default() != b {
		t.Fatalf("default not switched")
	}
}

func TestManagerConcurrentAccess(t *testing.T) {
	m := NewManager()
	_ = m.AddStore("a", newFake("a"))
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := context.Background()
			_ = m.PutString(ctx, "k", "v")
			_, _ = m.GetString(ctx, "k")
			_ = m.Forget(ctx, "k")
		}()
	}
	wg.Wait()
}
