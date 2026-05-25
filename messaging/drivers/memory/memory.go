package memory

import (
	"context"
	"strings"
	"sync"

	mdriver "github.com/godx-jp/godx-platform-framework/messaging/driver"
)

func init() {
	mdriver.Register(mdriver.DriverMemory, New)
}

func New(_ context.Context, _ mdriver.Spec) (mdriver.Broker, error) {
	return &broker{subs: map[string][]*sub{}}, nil
}

type sub struct {
	pattern string
	fn      mdriver.Handler
	cancel  func()
}

type broker struct {
	mu     sync.RWMutex
	closed bool
	subs   map[string][]*sub
}

func (b *broker) Name() string { return mdriver.DriverMemory }

func (b *broker) Publish(_ context.Context, subject string, msg mdriver.Message) error {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return mdriver.ErrClosed
	}
	msg.Subject = subject
	for _, list := range b.subs {
		for _, s := range list {
			if matchSubject(s.pattern, subject) {
				_ = s.fn(context.Background(), msg)
			}
		}
	}
	return nil
}

func (b *broker) Subscribe(_ context.Context, subject string, handler mdriver.Handler) (mdriver.Subscription, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil, mdriver.ErrClosed
	}
	s := &sub{pattern: subject, fn: handler}
	b.subs[subject] = append(b.subs[subject], s)
	return &subscription{cancel: func() { b.remove(s) }, s: s}, nil
}

func (b *broker) remove(target *sub) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for k, list := range b.subs {
		out := list[:0]
		for _, s := range list {
			if s != target {
				out = append(out, s)
			}
		}
		if len(out) == 0 {
			delete(b.subs, k)
		} else {
			b.subs[k] = out
		}
	}
}

func (b *broker) Close(context.Context) error {
	b.mu.Lock()
	b.closed = true
	b.subs = map[string][]*sub{}
	b.mu.Unlock()
	return nil
}

type subscription struct {
	cancel func()
	s      *sub
}

func (s *subscription) Cancel() error { s.cancel(); return nil }

func matchSubject(pattern, subject string) bool {
	if pattern == ">" || pattern == "*" {
		return true
	}
	if pattern == subject {
		return true
	}
	if strings.HasSuffix(pattern, ".>") {
		prefix := strings.TrimSuffix(pattern, ">")
		return strings.HasPrefix(subject, prefix)
	}
	return false
}
