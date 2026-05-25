package driver

import "context"

// Limiter is one rate-limit backend. Every method must be safe for
// concurrent use across goroutines.
type Limiter interface {
	// Name returns the canonical driver name.
	Name() string

	// Allow reports whether one token is available for key. When false,
	// the caller should treat the request as rate-limited.
	Allow(ctx context.Context, key string) (bool, error)

	// Reset clears the bucket state for key.
	Reset(ctx context.Context, key string)

	// Shutdown releases backend resources. Multiple calls must be safe.
	Shutdown(ctx context.Context) error
}

// Constructor builds a Limiter from Spec.
type Constructor func(ctx context.Context, spec Spec) (Limiter, error)
