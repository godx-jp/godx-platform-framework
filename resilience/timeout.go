package resilience

import (
	"context"
	"time"
)

// WithTimeout runs fn under a derived context deadline.
func WithTimeout(ctx context.Context, d time.Duration, fn func(context.Context) error) error {
	if d <= 0 {
		return fn(ctx)
	}
	cctx, cancel := context.WithTimeout(ctx, d)
	defer cancel()
	return fn(cctx)
}
