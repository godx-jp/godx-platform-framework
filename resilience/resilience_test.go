package resilience

import (
	"context"
	"errors"
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
