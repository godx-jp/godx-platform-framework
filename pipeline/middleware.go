package pipeline

import (
	"context"
	"net/http"
)

// HTTPStage is the net/http variant of Stage — an HTTP middleware
// callable. Provided as a convenience so callers can build typed
// pipelines around a *http.Request without losing access to the
// rest-of-chain handler.
type HTTPStage func(next http.Handler) http.Handler

// Chain composes HTTPStages right-to-left so the first argument
// runs outermost (matches the Laravel `Pipeline` ordering and most
// Go HTTP routers' `Use(...)` middleware contract).
func Chain(final http.Handler, stages ...HTTPStage) http.Handler {
	for i := len(stages) - 1; i >= 0; i-- {
		if stages[i] == nil {
			continue
		}
		final = stages[i](final)
	}
	return final
}

// FuncStage adapts a side-effect-only closure that runs every stage
// regardless of the next function — convenient for logging /
// metrics stages that always want to delegate.
func FuncStage[T any](fn func(ctx context.Context, value T)) Stage[T] {
	return func(ctx context.Context, value T, next Next[T]) (T, error) {
		fn(ctx, value)
		return next(ctx, value)
	}
}
