package resilience

import (
	"sync"
	"time"
)

// State is the observable state of a CircuitBreaker.
//
// The breaker is a leading indicator of dependency health: a transition
// to StateOpen means consecutive failures crossed the threshold and the
// breaker is now shedding load. Surfacing that transition (see
// CircuitBreakerConfig.OnStateChange) is the "early warning" operators
// need before the failure cascades.
type State int

const (
	// StateClosed: calls pass through normally.
	StateClosed State = iota
	// StateOpen: calls are rejected until ResetTimeout elapses.
	StateOpen
	// StateHalfOpen: ResetTimeout has elapsed; a single probe call is
	// allowed through. A successful probe closes the breaker, a failed
	// probe re-opens it.
	StateHalfOpen
)

// String returns the lowercase state name (e.g. "open").
func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreakerConfig configures failure thresholds.
type CircuitBreakerConfig struct {
	MaxFailures  int
	ResetTimeout time.Duration
	// OnStateChange, if non-nil, is invoked exactly once per actual
	// state transition with the previous and new state. It is called
	// outside the breaker's lock, so it is safe to log, emit metrics,
	// or call back into the breaker. nil (the default) keeps the
	// pre-Phase-4 behavior with zero overhead.
	OnStateChange func(from, to State)
}

// CircuitBreaker opens after MaxFailures consecutive failures.
type CircuitBreaker struct {
	mu            sync.Mutex
	failures      int
	maxFailures   int
	resetTimeout  time.Duration
	openUntil     time.Time
	state         State
	onStateChange func(from, to State)
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
	return &CircuitBreaker{
		maxFailures:   max,
		resetTimeout:  reset,
		state:         StateClosed,
		onStateChange: cfg.OnStateChange,
	}
}

// State returns the current observable state. Calling it may settle a
// lazy Open→HalfOpen transition if ResetTimeout has elapsed, firing
// OnStateChange like Allow would.
func (b *CircuitBreaker) State() State {
	b.mu.Lock()
	from, to := b.refreshLocked(time.Now())
	b.mu.Unlock()
	b.notify(from, to)
	return to
}

// Allow reports whether a call may proceed.
func (b *CircuitBreaker) Allow() error {
	b.mu.Lock()
	from, to := b.refreshLocked(time.Now())
	open := b.state == StateOpen
	b.mu.Unlock()
	b.notify(from, to)
	if open {
		return ErrOpen
	}
	return nil
}

// Success resets failure count and closes the breaker.
func (b *CircuitBreaker) Success() {
	b.mu.Lock()
	from := b.state
	b.failures = 0
	b.openUntil = time.Time{}
	b.state = StateClosed
	to := b.state
	b.mu.Unlock()
	b.notify(from, to)
}

// Failure records a failed call and may open the breaker.
func (b *CircuitBreaker) Failure() {
	b.mu.Lock()
	from := b.state
	b.failures++
	if b.failures >= b.maxFailures {
		b.openUntil = time.Now().Add(b.resetTimeout)
		b.state = StateOpen
	}
	to := b.state
	b.mu.Unlock()
	b.notify(from, to)
}

// refreshLocked settles a lazy Open→HalfOpen transition when the open
// window has elapsed. It must be called with b.mu held and returns the
// (from, to) states to hand to notify after the lock is released. If no
// transition occurred, from == to and notify is a no-op.
func (b *CircuitBreaker) refreshLocked(now time.Time) (from, to State) {
	from = b.state
	if b.state == StateOpen && !now.Before(b.openUntil) {
		b.state = StateHalfOpen
	}
	return from, b.state
}

// notify invokes OnStateChange outside the lock, but only on a real
// transition. Capturing from/to under the lock and calling here (never
// while holding b.mu) prevents a user callback that touches the breaker
// — or just blocks — from deadlocking it.
func (b *CircuitBreaker) notify(from, to State) {
	if b.onStateChange != nil && from != to {
		b.onStateChange(from, to)
	}
}
