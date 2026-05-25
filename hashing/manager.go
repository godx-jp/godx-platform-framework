package hashing

import (
	"context"
	"fmt"
	"sort"
	"sync"

	hdriver "github.com/godx-jp/godx-platform-framework/hashing/driver"
)

// Manager owns one or more named Hashers. Default() returns the
// store-flagged default; Hasher(name) returns a specific named
// hasher. Safe for concurrent use.
type Manager struct {
	mu       sync.RWMutex
	hashers  map[string]hdriver.Hasher
	def      string
	autodetect bool
}

// NewManager returns an empty Manager.
func NewManager() *Manager {
	return &Manager{hashers: map[string]hdriver.Hasher{}, autodetect: true}
}

// AddHasher registers h under name. Re-registering the same name
// returns an error.
func (m *Manager) AddHasher(name string, h hdriver.Hasher) error {
	if name == "" {
		return fmt.Errorf("hashing: AddHasher: name is required")
	}
	if h == nil {
		return fmt.Errorf("hashing: AddHasher(%q): nil hasher", name)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.hashers[name]; exists {
		return fmt.Errorf("hashing: hasher %q already registered", name)
	}
	m.hashers[name] = h
	if m.def == "" {
		m.def = name
	}
	return nil
}

// SetDefault flags name as the default hasher.
func (m *Manager) SetDefault(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.hashers[name]; !ok {
		return fmt.Errorf("hashing: SetDefault(%q): hasher not registered", name)
	}
	m.def = name
	return nil
}

// Default returns the default hasher.
func (m *Manager) Default() hdriver.Hasher {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.hashers[m.def]
}

// Hasher returns the named hasher.
func (m *Manager) Hasher(name string) (hdriver.Hasher, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	h, ok := m.hashers[name]
	if !ok {
		return nil, fmt.Errorf("hashing: hasher %q is not registered", name)
	}
	return h, nil
}

// Hashers returns the sorted list of registered hasher names.
func (m *Manager) Hashers() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]string, 0, len(m.hashers))
	for n := range m.hashers {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// CheckAny tries every registered hasher in turn against the encoded
// hash — useful for mixed deployments (legacy bcrypt + new argon2id).
// Returns the matching hasher's name on success.
func (m *Manager) CheckAny(ctx context.Context, plain, hash string) (bool, string, error) {
	m.mu.RLock()
	hashers := make([]struct {
		name string
		h    hdriver.Hasher
	}, 0, len(m.hashers))
	for n, h := range m.hashers {
		hashers = append(hashers, struct {
			name string
			h    hdriver.Hasher
		}{n, h})
	}
	m.mu.RUnlock()
	for _, item := range hashers {
		if _, err := item.h.Info(hash); err != nil {
			continue
		}
		ok, err := item.h.Check(ctx, plain, hash)
		if err != nil {
			return false, item.name, err
		}
		return ok, item.name, nil
	}
	return false, "", fmt.Errorf("hashing: no registered hasher recognises this encoded hash")
}

// Shutdown is a no-op today; hashers hold no resources. Present so
// the module fits framework lifecycle conventions without callers
// having to special-case it.
func (m *Manager) Shutdown(ctx context.Context) error { return nil }
