package resilience

import (
	"sync"
	"time"
)

// CircuitBreakerConfig configures failure thresholds.
type CircuitBreakerConfig struct {
	MaxFailures  int
	ResetTimeout time.Duration
}

// CircuitBreaker opens after MaxFailures consecutive failures.
type CircuitBreaker struct {
	mu           sync.Mutex
	failures     int
	maxFailures  int
	resetTimeout time.Duration
	openUntil    time.Time
}

// NewCircuitBreaker returns a breaker with sane defaults (5 failures, 30s reset).
func NewCircuitBreaker(cfg CircuitBreakerConfig) *CircuitBreaker {
	max := cfg.MaxFailures
	if max <= 0 {
		max = 5
	}
	reset := cfg.ResetTimeout
	if reset <= 0 {
		reset = 30 * time.Second
	}
	return &CircuitBreaker{maxFailures: max, resetTimeout: reset}
}

// Allow reports whether a call may proceed.
func (b *CircuitBreaker) Allow() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if time.Now().Before(b.openUntil) {
		return ErrOpen
	}
	return nil
}

// Success resets failure count.
func (b *CircuitBreaker) Success() {
	b.mu.Lock()
	b.failures = 0
	b.openUntil = time.Time{}
	b.mu.Unlock()
}

// Failure records a failed call and may open the breaker.
func (b *CircuitBreaker) Failure() {
	b.mu.Lock()
	b.failures++
	if b.failures >= b.maxFailures {
		b.openUntil = time.Now().Add(b.resetTimeout)
	}
	b.mu.Unlock()
}
