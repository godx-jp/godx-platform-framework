package resilience

import (
	"context"
	"errors"
	"math/rand"
	"time"
)

// ErrOpen is returned when a circuit breaker rejects a call.
var ErrOpen = errors.New("resilience: circuit breaker open")

// RetryConfig controls retry.Do behaviour.
type RetryConfig struct {
	MaxAttempts int
	Backoff     time.Duration
	Jitter      bool
	// Retryable decides whether fn's error warrants another attempt.
	// Nil retries every error except context cancellation.
	Retryable func(error) bool
}

// Do runs fn up to MaxAttempts times with optional backoff+jitter.
func Do(ctx context.Context, cfg RetryConfig, fn func(context.Context) error) error {
	attempts := cfg.MaxAttempts
	if attempts <= 0 {
		attempts = 1
	}
	backoff := cfg.Backoff
	if backoff <= 0 {
		backoff = 100 * time.Millisecond
	}
	retryable := cfg.Retryable
	if retryable == nil {
		retryable = func(err error) bool {
			return err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded)
		}
	}
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			wait := backoff
			if cfg.Jitter {
				wait += time.Duration(rand.Int63n(int64(backoff)))
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
		}
		lastErr = fn(ctx)
		if lastErr == nil {
			return nil
		}
		if !retryable(lastErr) {
			return lastErr
		}
	}
	return lastErr
}
