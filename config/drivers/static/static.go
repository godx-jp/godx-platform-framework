// Package static is the in-process map Source. It serves a fixed
// tree handed in at construction time — primarily useful for tests
// and for embedding compile-time defaults underneath other sources.
package static

import (
	"context"
	"sync"

	cdriver "github.com/godx-jp/godx-platform-framework/config/driver"
)

func init() {
	cdriver.Register(cdriver.DriverStatic, func(_ context.Context, spec cdriver.Spec) (cdriver.Source, error) {
		return &source{}, nil
	})
}

type source struct {
	mu     sync.RWMutex
	data   map[string]any
	closed bool
}

// New constructs a static Source seeded with data. The map is taken
// by reference; callers wanting isolation should clone first.
func New(data map[string]any) cdriver.Source {
	return &source{data: data}
}

func (s *source) Name() string { return cdriver.DriverStatic }

func (s *source) Load(ctx context.Context) (map[string]any, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, cdriver.ErrClosed
	}
	if s.data == nil {
		return map[string]any{}, nil
	}
	return s.data, nil
}

// Update replaces the served tree atomically.
func (s *source) Update(data map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = data
}

func (s *source) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	return nil
}
