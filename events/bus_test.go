package events

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestListenDispatchExact(t *testing.T) {
	bus := New()
	defer func() { _ = bus.Close(context.Background()) }()
	var got int
	bus.Listen("user.created", func(ctx context.Context, e Event) error {
		if e.Name != "user.created" {
			t.Errorf("event name unexpected: %q", e.Name)
		}
		got = e.Payload.(int)
		return nil
	})
	if err := bus.Dispatch(context.Background(), Event{Name: "user.created", Payload: 42}); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if got != 42 {
		t.Fatalf("listener did not run: got %d", got)
	}
}

func TestDispatchWildcardFanout(t *testing.T) {
	bus := New()
	defer func() { _ = bus.Close(context.Background()) }()
	var n atomic.Int32
	bus.Listen("user.*", func(ctx context.Context, e Event) error { n.Add(1); return nil })
	bus.Listen("*", func(ctx context.Context, e Event) error { n.Add(10); return nil })
	bus.Listen("user.created", func(ctx context.Context, e Event) error { n.Add(100); return nil })
	bus.Listen("order.*", func(ctx context.Context, e Event) error { n.Add(1000); return nil })

	_ = bus.Dispatch(context.Background(), Event{Name: "user.created"})
	if got := n.Load(); got != 111 {
		t.Fatalf("fanout count unexpected: %d", got)
	}
}

func TestDispatchTimestampDefault(t *testing.T) {
	bus := New()
	defer func() { _ = bus.Close(context.Background()) }()
	var stamped time.Time
	bus.Listen("x", func(ctx context.Context, e Event) error {
		stamped = e.CreatedAt
		return nil
	})
	before := time.Now()
	_ = bus.Dispatch(context.Background(), Event{Name: "x"})
	if stamped.IsZero() || stamped.Before(before) {
		t.Fatalf("CreatedAt not stamped: %v (before=%v)", stamped, before)
	}
}

func TestDispatchListenerErrorJoined(t *testing.T) {
	bus := New()
	defer func() { _ = bus.Close(context.Background()) }()
	errA := errors.New("boom A")
	errB := errors.New("boom B")
	bus.Listen("x", func(ctx context.Context, e Event) error { return errA })
	bus.Listen("x", func(ctx context.Context, e Event) error { return errB })
	bus.Listen("x", func(ctx context.Context, e Event) error { return nil })

	err := bus.Dispatch(context.Background(), Event{Name: "x"})
	if err == nil {
		t.Fatalf("expected joined error")
	}
	if !errors.Is(err, errA) || !errors.Is(err, errB) {
		t.Fatalf("joined error missing children: %v", err)
	}
}

func TestDispatchPanicRecovered(t *testing.T) {
	bus := New()
	defer func() { _ = bus.Close(context.Background()) }()
	bus.Listen("x", func(ctx context.Context, e Event) error {
		panic("oh no")
	})
	bus.Listen("x", func(ctx context.Context, e Event) error { return nil })
	err := bus.Dispatch(context.Background(), Event{Name: "x"})
	if err == nil {
		t.Fatalf("expected panic-converted error")
	}
}

func TestSubscriptionCancel(t *testing.T) {
	bus := New()
	defer func() { _ = bus.Close(context.Background()) }()
	var n atomic.Int32
	s := bus.Listen("x", func(ctx context.Context, e Event) error { n.Add(1); return nil })
	_ = bus.Dispatch(context.Background(), Event{Name: "x"})
	s.Cancel()
	_ = bus.Dispatch(context.Background(), Event{Name: "x"})
	if got := n.Load(); got != 1 {
		t.Fatalf("Cancel did not stop subsequent dispatches: %d", got)
	}
	s.Cancel() // double-cancel safe
}

func TestForgetPattern(t *testing.T) {
	bus := New()
	defer func() { _ = bus.Close(context.Background()) }()
	bus.Listen("x", func(ctx context.Context, e Event) error { return nil })
	bus.Listen("x", func(ctx context.Context, e Event) error { return nil })
	bus.Listen("y", func(ctx context.Context, e Event) error { return nil })
	if got := bus.Forget("x"); got != 2 {
		t.Fatalf("Forget returned %d, want 2", got)
	}
	if got := len(bus.Patterns()); got != 1 {
		t.Fatalf("after Forget, %d patterns left", got)
	}
}

func TestDispatchEmptyNameRejected(t *testing.T) {
	bus := New()
	defer func() { _ = bus.Close(context.Background()) }()
	if err := bus.Dispatch(context.Background(), Event{Name: ""}); err == nil {
		t.Fatalf("empty name should be rejected")
	}
}

func TestListenEmptyPatternPanics(t *testing.T) {
	bus := New()
	defer func() { _ = bus.Close(context.Background()) }()
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic on empty pattern")
		}
	}()
	bus.Listen("", func(ctx context.Context, e Event) error { return nil })
}

func TestListenNilListenerPanics(t *testing.T) {
	bus := New()
	defer func() { _ = bus.Close(context.Background()) }()
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic on nil Listener")
		}
	}()
	bus.Listen("x", nil)
}

func TestDispatchContextCanceled(t *testing.T) {
	bus := New()
	defer func() { _ = bus.Close(context.Background()) }()
	bus.Listen("x", func(ctx context.Context, e Event) error {
		<-ctx.Done()
		return ctx.Err()
	})
	bus.Listen("x", func(ctx context.Context, e Event) error { return nil })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := bus.Dispatch(ctx, Event{Name: "x"})
	if err == nil {
		t.Fatalf("expected context error")
	}
}

func TestCloseIdempotentAndRejectsDispatch(t *testing.T) {
	bus := New()
	if err := bus.Close(context.Background()); err != nil {
		t.Fatalf("close 1: %v", err)
	}
	if err := bus.Close(context.Background()); err != nil {
		t.Fatalf("close 2 should be safe: %v", err)
	}
	if err := bus.Dispatch(context.Background(), Event{Name: "x"}); !errors.Is(err, ErrClosed) {
		t.Fatalf("expected ErrClosed, got %v", err)
	}
}

func TestConcurrentListenDispatch(t *testing.T) {
	bus := New()
	defer func() { _ = bus.Close(context.Background()) }()
	var n atomic.Int64
	for i := 0; i < 10; i++ {
		i := i
		bus.Listen(fmt.Sprintf("p%d", i), func(ctx context.Context, e Event) error {
			n.Add(1)
			return nil
		})
	}
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		i := i
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = bus.Dispatch(context.Background(), Event{Name: fmt.Sprintf("p%d", i%10)})
		}()
		go func() {
			defer wg.Done()
			s := bus.Listen("transient", func(ctx context.Context, e Event) error { return nil })
			s.Cancel()
		}()
	}
	wg.Wait()
	if n.Load() != 100 {
		t.Fatalf("expected 100 dispatches to land, got %d", n.Load())
	}
}
