package secrets

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"

	sdriver "github.com/godx-jp/godx-platform-framework/secrets/driver"
)

// Manager owns one or more named secret stores. Default() returns
// the manager's default store; Store(name) returns a specific one.
// Safe for concurrent use.
type Manager struct {
	mu     sync.RWMutex
	stores map[string]sdriver.Store
	def    string
}

// NewManager returns an empty Manager.
func NewManager() *Manager {
	return &Manager{stores: map[string]sdriver.Store{}}
}

// AddStore registers s under name. The first registered store is
// flagged as default; call SetDefault to switch.
func (m *Manager) AddStore(name string, s sdriver.Store) error {
	if name == "" {
		return fmt.Errorf("secrets: AddStore: name is required")
	}
	if s == nil {
		return fmt.Errorf("secrets: AddStore(%q): nil store", name)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.stores[name]; exists {
		return fmt.Errorf("secrets: store %q already registered", name)
	}
	m.stores[name] = s
	if m.def == "" {
		m.def = name
	}
	return nil
}

// SetDefault flags name as the default store.
func (m *Manager) SetDefault(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.stores[name]; !ok {
		return fmt.Errorf("secrets: SetDefault(%q): store not registered", name)
	}
	m.def = name
	return nil
}

// Default returns the default store. nil when none are registered.
func (m *Manager) Default() sdriver.Store {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stores[m.def]
}

// Store returns the named store, or an error if not registered.
func (m *Manager) Store(name string) (sdriver.Store, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.stores[name]
	if !ok {
		return nil, fmt.Errorf("secrets: store %q is not registered", name)
	}
	return s, nil
}

// Stores returns the sorted list of registered store names.
func (m *Manager) Stores() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]string, 0, len(m.stores))
	for n := range m.stores {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Get reads key from the default store.
func (m *Manager) Get(ctx context.Context, key string) ([]byte, error) {
	d := m.Default()
	if d == nil {
		return nil, fmt.Errorf("secrets: no default store configured")
	}
	return d.Get(ctx, key)
}

// GetString reads key from the default store and returns it as a
// string.
func (m *Manager) GetString(ctx context.Context, key string) (string, error) {
	b, err := m.Get(ctx, key)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Put writes key to the default store.
func (m *Manager) Put(ctx context.Context, key string, value []byte) error {
	d := m.Default()
	if d == nil {
		return fmt.Errorf("secrets: no default store configured")
	}
	return d.Put(ctx, key, value)
}

// PutString writes a string value to the default store.
func (m *Manager) PutString(ctx context.Context, key, value string) error {
	return m.Put(ctx, key, []byte(value))
}

// Forget removes key from the default store.
func (m *Manager) Forget(ctx context.Context, key string) error {
	d := m.Default()
	if d == nil {
		return fmt.Errorf("secrets: no default store configured")
	}
	return d.Forget(ctx, key)
}

// Shutdown shuts every registered store down, returning the joined
// error of all non-nil per-store errors.
func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	stores := make([]sdriver.Store, 0, len(m.stores))
	for _, s := range m.stores {
		stores = append(stores, s)
	}
	m.mu.Unlock()
	var errs []error
	for _, s := range stores {
		if err := s.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
