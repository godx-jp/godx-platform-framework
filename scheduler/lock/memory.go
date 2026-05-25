package lock

import (
	"context"
	"sync"
)

// Memory is a process-local mutex map for WithoutOverlapping.
type Memory struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

// NewMemory returns an empty in-process lock table.
func NewMemory() *Memory {
	return &Memory{locks: map[string]*sync.Mutex{}}
}

// TryAcquire grabs key. ok is false when another goroutine holds it.
func (m *Memory) TryAcquire(_ context.Context, key string) (func() error, bool, error) {
	m.mu.Lock()
	lk, ok := m.locks[key]
	if !ok {
		lk = &sync.Mutex{}
		m.locks[key] = lk
	}
	m.mu.Unlock()

	if !lk.TryLock() {
		return nil, false, nil
	}
	return func() error {
		lk.Unlock()
		return nil
	}, true, nil
}
