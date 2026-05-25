package observability

import (
	"context"

	"github.com/godx-jp/godx-platform-framework/framework"
)

// StoreKey is the [framework.App.Store] key under which the active
// [*Provider] is registered by the observability module.
const StoreKey = "observability.provider"

type providerCtxKey struct{}
type correlationCtxKey struct{}

// ContextWithProvider returns ctx augmented with p so that [FromContext] can
// recover it. Useful in worker contexts that lack an [framework.App].
func ContextWithProvider(ctx context.Context, p *Provider) context.Context {
	return context.WithValue(ctx, providerCtxKey{}, p)
}

// FromContext recovers the provider stored by [ContextWithProvider] or the
// HTTP middleware. Returns the no-op global if none is set, so call sites do
// not need nil checks.
func FromContext(ctx context.Context) *Provider {
	if p, ok := ctx.Value(providerCtxKey{}).(*Provider); ok && p != nil {
		return p
	}
	return globalProvider()
}

// FromApp recovers the provider from a framework [App] store. Panics if the
// observability module is not registered — services should call this once at
// startup and then propagate through context.
func FromApp(app *framework.App) *Provider {
	v, ok := app.Lookup(StoreKey)
	if !ok {
		panic("observability: module not registered on app; call app.Use(observability.Module)")
	}
	p, ok := v.(*Provider)
	if !ok {
		panic("observability: stored value is not *Provider")
	}
	return p
}

// ContextWithCorrelationID stores the given correlation ID on ctx so that
// log records emitted later carry it automatically.
func ContextWithCorrelationID(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, correlationCtxKey{}, id)
}

// CorrelationIDFromContext returns the correlation ID stored on ctx by
// [ContextWithCorrelationID] or the HTTP middleware. Returns "" when unset.
func CorrelationIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(correlationCtxKey{}).(string); ok {
		return v
	}
	return ""
}
