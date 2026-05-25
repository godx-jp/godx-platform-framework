package cache

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
)

// Manager holds a map of named Stores and exposes a Laravel-style
// `Cache::store(name)` API. One Store per backend; the default store
// is returned by Default / DefaultName.
type Manager struct {
	mu          sync.RWMutex
	defaultName string
	stores      map[string]*Store
}

// NewManager returns an empty Manager. Production code goes through
// cache.Module / cache.ModuleWithConfig; tests can construct directly.
func NewManager() *Manager {
	return &Manager{stores: map[string]*Store{}}
}

// AddStore registers s under its current Name. Returns an error if a
// store with the same name is already present (use a fresh Manager or
// swap explicitly).
func (m *Manager) AddStore(s *Store) error {
	if s == nil {
		return errors.New("cache: AddStore called with nil store")
	}
	if s.Name() == "" {
		return errors.New("cache: AddStore called with empty store name")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, dup := m.stores[s.Name()]; dup {
		return fmt.Errorf("cache: store %q already registered", s.Name())
	}
	m.stores[s.Name()] = s
	return nil
}

// SetDefault marks name as the default store returned by Default().
// Returns an error when the named store is not registered.
func (m *Manager) SetDefault(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.stores[name]; !ok {
		return fmt.Errorf("cache: default store %q is not registered", name)
	}
	m.defaultName = name
	return nil
}

// Store returns the named store. Returns an error when the store is
// not registered.
func (m *Manager) Store(name string) (*Store, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.stores[name]
	if !ok {
		return nil, fmt.Errorf("cache: store %q not registered", name)
	}
	return s, nil
}

// MustStore is the panicking variant of Store, useful for top-level
// boot wiring where a missing store is a programmer error.
func (m *Manager) MustStore(name string) *Store {
	s, err := m.Store(name)
	if err != nil {
		panic(err)
	}
	return s
}

// Default returns the default store. Panics when no default has been
// set yet (Manager constructed by hand without a SetDefault call).
func (m *Manager) Default() *Store {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.defaultName == "" {
		panic("cache: Default called before SetDefault")
	}
	return m.stores[m.defaultName]
}

// DefaultName returns the configured default store name.
func (m *Manager) DefaultName() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.defaultName
}

// Stores returns the sorted list of registered store names. Useful for
// diagnostics endpoints (e.g. /healthz emitting which caches are wired).
func (m *Manager) Stores() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]string, 0, len(m.stores))
	for name := range m.stores {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// Shutdown calls Shutdown on every registered store. Errors are joined
// so a failure on one store does not skip the others.
func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	stores := m.stores
	m.stores = map[string]*Store{}
	m.defaultName = ""
	m.mu.Unlock()

	var errs []error
	for name, s := range stores {
		if err := s.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("cache: shutdown %q: %w", name, err))
		}
	}
	return errors.Join(errs...)
}
