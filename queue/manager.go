package queue

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// Manager owns one or more named Queue connections.
type Manager struct {
	mu       sync.RWMutex
	queues   map[string]*Queue
	defaultQ string
}

func NewManager() *Manager {
	return &Manager{queues: make(map[string]*Queue)}
}

func (m *Manager) AddQueue(q *Queue) error {
	if q == nil {
		return fmt.Errorf("queue: nil Queue")
	}
	name := q.Name()
	if name == "" {
		return fmt.Errorf("queue: Queue name is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.queues[name]; exists {
		return fmt.Errorf("queue: duplicate queue %q", name)
	}
	m.queues[name] = q
	if m.defaultQ == "" {
		m.defaultQ = name
	}
	return nil
}

func (m *Manager) SetDefault(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.queues[name]; !ok {
		return fmt.Errorf("queue: unknown queue %q", name)
	}
	m.defaultQ = name
	return nil
}

func (m *Manager) Default() (*Queue, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.defaultQ == "" {
		return nil, fmt.Errorf("queue: no default queue configured")
	}
	return m.queues[m.defaultQ], nil
}

func (m *Manager) Queue(name string) (*Queue, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	q, ok := m.queues[name]
	if !ok {
		return nil, fmt.Errorf("queue: unknown queue %q", name)
	}
	return q, nil
}

func (m *Manager) Queues() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]string, 0, len(m.queues))
	for n := range m.queues {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.RLock()
	qs := make([]*Queue, 0, len(m.queues))
	for _, q := range m.queues {
		qs = append(qs, q)
	}
	m.mu.RUnlock()
	var first error
	for _, q := range qs {
		if err := q.Shutdown(ctx); err != nil && first == nil {
			first = err
		}
	}
	return first
}
