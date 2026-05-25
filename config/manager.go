package config

import (
	"context"
	"errors"
	"fmt"
	"sync"

	cdriver "github.com/godx-jp/godx-platform-framework/config/driver"
)

// Manager owns a chain of Sources and exposes the merged Repository.
// Sources are merged in registration order — the last source wins on
// overlapping keys. Manager itself is safe for concurrent use.
type Manager struct {
	mu      sync.RWMutex
	closed  bool
	sources []namedSource
	repo    *Repository
	subs    []func(*Repository)
}

type namedSource struct {
	name   string
	source cdriver.Source
}

// NewManager returns an empty Manager. AddSource feeds it Sources;
// Reload merges them into the Repository.
func NewManager() *Manager {
	return &Manager{repo: NewRepository(nil)}
}

// Repository returns the typed accessor over the merged tree. The
// returned pointer is stable across reloads — the underlying data
// is swapped in place — so callers may cache it for the lifetime of
// the Manager.
func (m *Manager) Repository() *Repository {
	return m.repo
}

// Sources returns the registered source names in order.
func (m *Manager) Sources() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]string, 0, len(m.sources))
	for _, s := range m.sources {
		out = append(out, s.name)
	}
	return out
}

// AddSource registers s under name and merges its current tree on
// top of the existing Repository. Reload re-runs Load across every
// registered source.
func (m *Manager) AddSource(ctx context.Context, name string, s cdriver.Source) error {
	if s == nil {
		return fmt.Errorf("config: AddSource(%q): nil source", name)
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return cdriver.ErrClosed
	}
	for _, existing := range m.sources {
		if existing.name == name {
			m.mu.Unlock()
			return fmt.Errorf("config: source %q already registered", name)
		}
	}
	m.sources = append(m.sources, namedSource{name: name, source: s})
	m.mu.Unlock()

	if err := m.Reload(ctx); err != nil {
		m.mu.Lock()
		m.sources = m.sources[:len(m.sources)-1]
		m.mu.Unlock()
		return fmt.Errorf("config: source %q reload: %w", name, err)
	}

	if w, ok := s.(cdriver.Watcher); ok {
		if err := w.Watch(ctx, func() {
			_ = m.Reload(ctx)
		}); err != nil && !errors.Is(err, cdriver.ErrNotSupported) {
			return fmt.Errorf("config: source %q watch: %w", name, err)
		}
	}
	return nil
}

// Reload re-runs Load on every Source and rebuilds the Repository's
// underlying tree. Returns the first error encountered; subsequent
// sources are still attempted so a single bad source does not poison
// the chain.
func (m *Manager) Reload(ctx context.Context) error {
	m.mu.RLock()
	if m.closed {
		m.mu.RUnlock()
		return cdriver.ErrClosed
	}
	sources := make([]namedSource, len(m.sources))
	copy(sources, m.sources)
	m.mu.RUnlock()

	merged := map[string]any{}
	var firstErr error
	for _, s := range sources {
		data, err := s.source.Load(ctx)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("config: source %q load: %w", s.name, err)
			}
			continue
		}
		merged = Merge(merged, data)
	}
	m.repo.replace(merged)
	m.fireSubs()
	return firstErr
}

// OnChange registers cb to fire after every Reload. cb runs
// synchronously inside the reload — keep it cheap.
func (m *Manager) OnChange(cb func(*Repository)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.subs = append(m.subs, cb)
}

func (m *Manager) fireSubs() {
	m.mu.RLock()
	subs := make([]func(*Repository), len(m.subs))
	copy(subs, m.subs)
	m.mu.RUnlock()
	for _, cb := range subs {
		cb(m.repo)
	}
}

// Shutdown stops every Source. Subsequent calls return ErrClosed.
func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil
	}
	m.closed = true
	sources := m.sources
	m.sources = nil
	m.mu.Unlock()

	var firstErr error
	for _, s := range sources {
		if err := s.source.Shutdown(ctx); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("config: source %q shutdown: %w", s.name, err)
		}
	}
	return firstErr
}
