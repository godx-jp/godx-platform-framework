package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"

	rdriver "github.com/godx-jp/godx-platform-framework/ratelimit/driver"
)

// Manager owns named rate limiters.
type Manager struct {
	mu       sync.RWMutex
	limiters map[string]rdriver.Limiter
	def      string
}

func NewManager() *Manager {
	return &Manager{limiters: map[string]rdriver.Limiter{}}
}

func (m *Manager) AddLimiter(name string, l rdriver.Limiter) error {
	if name == "" || l == nil {
		return fmt.Errorf("ratelimit: AddLimiter: name and limiter required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.limiters[name]; exists {
		return fmt.Errorf("ratelimit: limiter %q already registered", name)
	}
	m.limiters[name] = l
	if m.def == "" {
		m.def = name
	}
	return nil
}

func (m *Manager) SetDefault(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.limiters[name]; !ok {
		return fmt.Errorf("ratelimit: SetDefault(%q): not registered", name)
	}
	m.def = name
	return nil
}

func (m *Manager) Default() rdriver.Limiter {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.limiters[m.def]
}

func (m *Manager) Limiter(name string) (rdriver.Limiter, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	l, ok := m.limiters[name]
	if !ok {
		return nil, fmt.Errorf("ratelimit: limiter %q not registered", name)
	}
	return l, nil
}

func (m *Manager) Names() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]string, 0, len(m.limiters))
	for n := range m.limiters {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

func (m *Manager) Allow(ctx context.Context, key string) (bool, error) {
	d := m.Default()
	if d == nil {
		return false, fmt.Errorf("ratelimit: no default limiter")
	}
	return d.Allow(ctx, key)
}

func (m *Manager) Reset(ctx context.Context, key string) {
	d := m.Default()
	if d != nil {
		d.Reset(ctx, key)
	}
}

func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	limiters := make([]rdriver.Limiter, 0, len(m.limiters))
	for _, l := range m.limiters {
		limiters = append(limiters, l)
	}
	m.mu.Unlock()
	var errs []error
	for _, l := range limiters {
		if err := l.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
