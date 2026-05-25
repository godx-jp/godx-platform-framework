package events

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Bus is the public dispatcher interface. The default in-process
// implementation lives in this file; the async wrapper lives under
// events/async/.
type Bus interface {
	Listen(pattern string, l Listener) Subscription
	Forget(pattern string) int
	Dispatch(ctx context.Context, e Event) error
	Patterns() []string
	Close(ctx context.Context) error
}

// ErrClosed is returned by a closed Bus.
var ErrClosed = errors.New("events: bus is closed")

type subscription struct {
	id      uint64
	pattern string
	fn      Listener
	bus     *syncBus
}

func (s *subscription) Cancel() {
	if s == nil || s.bus == nil {
		return
	}
	s.bus.unsubscribe(s.id)
}

func (s *subscription) Pattern() string { return s.pattern }

// syncBus is the in-process synchronous Bus.
type syncBus struct {
	mu       sync.RWMutex
	closed   bool
	nextID   atomic.Uint64
	subs     []*subscription
}

// New returns a new in-process synchronous Bus.
func New() Bus { return &syncBus{} }

func (b *syncBus) Listen(pattern string, l Listener) Subscription {
	if l == nil {
		panic("events: Listen called with nil Listener")
	}
	if pattern == "" {
		panic("events: Listen called with empty pattern")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return &subscription{pattern: pattern}
	}
	s := &subscription{
		id:      b.nextID.Add(1),
		pattern: pattern,
		fn:      l,
		bus:     b,
	}
	b.subs = append(b.subs, s)
	return s
}

func (b *syncBus) unsubscribe(id uint64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i, s := range b.subs {
		if s.id == id {
			b.subs = append(b.subs[:i], b.subs[i+1:]...)
			return
		}
	}
}

func (b *syncBus) Forget(pattern string) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	kept := b.subs[:0]
	removed := 0
	for _, s := range b.subs {
		if s.pattern == pattern {
			removed++
			continue
		}
		kept = append(kept, s)
	}
	b.subs = kept
	return removed
}

func (b *syncBus) Dispatch(ctx context.Context, e Event) error {
	if e.Name == "" {
		return fmt.Errorf("events: Event.Name is required")
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now()
	}
	b.mu.RLock()
	if b.closed {
		b.mu.RUnlock()
		return ErrClosed
	}
	matches := make([]*subscription, 0, len(b.subs))
	for _, s := range b.subs {
		if match(s.pattern, e.Name) {
			matches = append(matches, s)
		}
	}
	b.mu.RUnlock()

	var errs []error
	for _, s := range matches {
		if err := ctx.Err(); err != nil {
			errs = append(errs, err)
			break
		}
		if err := safeInvoke(ctx, s.fn, e); err != nil {
			errs = append(errs, fmt.Errorf("events: listener %q for event %q: %w", s.pattern, e.Name, err))
		}
	}
	return errors.Join(errs...)
}

func safeInvoke(ctx context.Context, l Listener, e Event) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("listener panic: %v", r)
		}
	}()
	return l(ctx, e)
}

func (b *syncBus) Patterns() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]string, 0, len(b.subs))
	for _, s := range b.subs {
		out = append(out, s.pattern)
	}
	return out
}

func (b *syncBus) Close(ctx context.Context) error {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil
	}
	b.closed = true
	b.subs = nil
	b.mu.Unlock()
	return nil
}
