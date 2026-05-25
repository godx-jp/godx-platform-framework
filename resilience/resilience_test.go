package resilience

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRetrySucceedsEventually(t *testing.T) {
	var n atomic.Int32
	err := Do(context.Background(), RetryConfig{MaxAttempts: 3, Backoff: time.Millisecond}, func(context.Context) error {
		if n.Add(1) < 3 {
			return errors.New("transient")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
}

func TestCircuitBreakerOpens(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{MaxFailures: 2, ResetTimeout: time.Hour})
	cb.Failure()
	if err := cb.Allow(); err != nil {
		t.Fatalf("should stay closed: %v", err)
	}
	cb.Failure()
	if err := cb.Allow(); err != ErrOpen {
		t.Fatalf("want open, got %v", err)
	}
	cb.Success()
	if err := cb.Allow(); err != nil {
		t.Fatalf("should close after success: %v", err)
	}
}

func TestCircuitBreakerOnStateChange(t *testing.T) {
	var mu sync.Mutex
	var transitions [][2]State
	record := func(from, to State) {
		mu.Lock()
		transitions = append(transitions, [2]State{from, to})
		mu.Unlock()
	}
	snapshot := func() [][2]State {
		mu.Lock()
		defer mu.Unlock()
		return append([][2]State(nil), transitions...)
	}

	cb := NewCircuitBreaker(CircuitBreakerConfig{
		MaxFailures:   2,
		ResetTimeout:  20 * time.Millisecond,
		OnStateChange: record,
	})

	// Below threshold: no transition.
	cb.Failure()
	if got := snapshot(); len(got) != 0 {
		t.Fatalf("no transition expected before threshold, got %v", got)
	}

	// Closed -> Open at MaxFailures.
	cb.Failure()
	if got := snapshot(); len(got) != 1 || got[0] != [2]State{StateClosed, StateOpen} {
		t.Fatalf("want Closed->Open, got %v", got)
	}

	// Same-state op while open must not re-fire.
	if err := cb.Allow(); err != ErrOpen {
		t.Fatalf("want open, got %v", err)
	}
	if got := snapshot(); len(got) != 1 {
		t.Fatalf("Allow while open must not re-fire, got %v", got)
	}

	// Open -> HalfOpen once ResetTimeout elapses (probe allowed).
	time.Sleep(30 * time.Millisecond)
	if err := cb.Allow(); err != nil {
		t.Fatalf("probe should be allowed after reset: %v", err)
	}
	if got := snapshot(); len(got) != 2 || got[1] != [2]State{StateOpen, StateHalfOpen} {
		t.Fatalf("want Open->HalfOpen, got %v", got)
	}

	// HalfOpen -> Closed on a successful probe.
	cb.Success()
	if got := snapshot(); len(got) != 3 || got[2] != [2]State{StateHalfOpen, StateClosed} {
		t.Fatalf("want HalfOpen->Closed, got %v", got)
	}

	// Success while already closed must not fire.
	cb.Success()
	if got := snapshot(); len(got) != 3 {
		t.Fatalf("Success while closed must not re-fire, got %v", got)
	}
}

func TestCircuitBreakerNilCallbackSafe(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{MaxFailures: 1, ResetTimeout: time.Millisecond})
	cb.Failure() // would fire OnStateChange if it were set
	if err := cb.Allow(); err != ErrOpen {
		t.Fatalf("want open, got %v", err)
	}
	cb.Success()
	if cb.State() != StateClosed {
		t.Fatalf("want closed, got %v", cb.State())
	}
}

func TestStateString(t *testing.T) {
	cases := map[State]string{
		StateClosed:   "closed",
		StateOpen:     "open",
		StateHalfOpen: "half-open",
		State(99):     "unknown",
	}
	for s, want := range cases {
		if got := s.String(); got != want {
			t.Fatalf("State(%d).String() = %q, want %q", s, got, want)
		}
	}
}

// TestCircuitBreakerConcurrentNoDeadlock fires the breaker from many
// goroutines while OnStateChange calls back into the breaker. If the
// callback were invoked under the breaker's lock this would deadlock;
// run under -race to also prove no data race on the state field.
func TestCircuitBreakerConcurrentNoDeadlock(t *testing.T) {
	var calls atomic.Int64
	var cb *CircuitBreaker
	cb = NewCircuitBreaker(CircuitBreakerConfig{
		MaxFailures:  3,
		ResetTimeout: time.Millisecond,
		OnStateChange: func(from, to State) {
			calls.Add(1)
			// Re-enter the breaker from inside the callback: only
			// safe because notify runs outside the lock.
			_ = cb.State()
		},
	})

	done := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
					cb.Failure()
					_ = cb.Allow()
					cb.Success()
				}
			}
		}()
	}
	time.Sleep(50 * time.Millisecond)
	close(done)
	wg.Wait()
	if calls.Load() == 0 {
		t.Fatal("expected at least one state-change callback")
	}
}

func TestWithTimeout(t *testing.T) {
	err := WithTimeout(context.Background(), 10*time.Millisecond, func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
			return nil
		}
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err=%v", err)
	}
}

func TestBulkheadLimitsConcurrency(t *testing.T) {
	b := NewBulkhead(1)
	release, err := b.Acquire(context.Background())
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	_, err = b.Acquire(context.Background())
	if !errors.Is(err, ErrBulkheadFull) {
		t.Fatalf("want full, got %v", err)
	}
	release()
}
