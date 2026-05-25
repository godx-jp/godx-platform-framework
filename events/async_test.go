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

func TestAsyncDispatchFiresAndForgets(t *testing.T) {
	inner := New()
	bus := NewAsync(inner, AsyncOptions{Workers: 2, QueueSize: 16})
	defer func() { _ = bus.Close(context.Background()) }()

	var n atomic.Int64
	bus.Listen("x", func(ctx context.Context, e Event) error {
		n.Add(1)
		return nil
	})
	for i := 0; i < 100; i++ {
		if err := bus.Dispatch(context.Background(), Event{Name: "x"}); err != nil {
			t.Fatalf("dispatch %d: %v", i, err)
		}
	}
	deadline := time.After(2 * time.Second)
	for n.Load() < 100 {
		select {
		case <-deadline:
			t.Fatalf("only %d dispatches landed", n.Load())
		default:
			time.Sleep(time.Millisecond)
		}
	}
}

func TestAsyncOnErrorReceivesListenerError(t *testing.T) {
	inner := New()
	var errs []error
	var mu sync.Mutex
	bus := NewAsync(inner, AsyncOptions{
		Workers: 1,
		OnError: func(err error) {
			mu.Lock()
			errs = append(errs, err)
			mu.Unlock()
		},
	})
	defer func() { _ = bus.Close(context.Background()) }()
	expected := errors.New("listener boom")
	bus.Listen("x", func(ctx context.Context, e Event) error { return expected })

	for i := 0; i < 5; i++ {
		_ = bus.Dispatch(context.Background(), Event{Name: "x"})
	}
	deadline := time.After(2 * time.Second)
	for {
		mu.Lock()
		n := len(errs)
		mu.Unlock()
		if n >= 5 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("expected 5 errors, got %d", n)
		default:
			time.Sleep(time.Millisecond)
		}
	}
	mu.Lock()
	for _, e := range errs {
		if !errors.Is(e, expected) {
			t.Fatalf("error missing original: %v", e)
		}
	}
	mu.Unlock()
}

func TestAsyncCloseDrainsPending(t *testing.T) {
	inner := New()
	bus := NewAsync(inner, AsyncOptions{Workers: 1, QueueSize: 64})

	var n atomic.Int64
	bus.Listen("x", func(ctx context.Context, e Event) error {
		time.Sleep(time.Millisecond)
		n.Add(1)
		return nil
	})
	for i := 0; i < 32; i++ {
		_ = bus.Dispatch(context.Background(), Event{Name: "x"})
	}
	if err := bus.Close(context.Background()); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if got := n.Load(); got < 32 {
		t.Fatalf("Close did not drain: %d landed of 32", got)
	}
}

func TestAsyncDispatchAfterCloseRejected(t *testing.T) {
	inner := New()
	bus := NewAsync(inner, AsyncOptions{Workers: 1})
	_ = bus.Close(context.Background())
	err := bus.Dispatch(context.Background(), Event{Name: "x"})
	if !errors.Is(err, ErrClosed) {
		t.Fatalf("expected ErrClosed, got %v", err)
	}
}

func TestAsyncDispatchContextCancelBeforeAccept(t *testing.T) {
	inner := New()
	bus := NewAsync(inner, AsyncOptions{Workers: 1, QueueSize: 1})
	defer func() { _ = bus.Close(context.Background()) }()

	bus.Listen("hold", func(ctx context.Context, e Event) error {
		time.Sleep(50 * time.Millisecond)
		return nil
	})
	_ = bus.Dispatch(context.Background(), Event{Name: "hold"})
	_ = bus.Dispatch(context.Background(), Event{Name: "hold"})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	err := bus.Dispatch(ctx, Event{Name: "hold"})
	if err == nil {
		t.Fatalf("expected context error when queue full")
	}
}

func TestAsyncProxiesListenForgetPatterns(t *testing.T) {
	inner := New()
	bus := NewAsync(inner, AsyncOptions{Workers: 1})
	defer func() { _ = bus.Close(context.Background()) }()
	for i := 0; i < 3; i++ {
		bus.Listen(fmt.Sprintf("p%d", i), func(ctx context.Context, e Event) error { return nil })
	}
	if got := len(bus.Patterns()); got != 3 {
		t.Fatalf("Patterns count: %d", got)
	}
	if got := bus.Forget("p0"); got != 1 {
		t.Fatalf("Forget p0 returned %d", got)
	}
}
