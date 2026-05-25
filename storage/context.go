package storage

import (
	"context"

	"github.com/godx-jp/godx-platform-framework/framework"
)

type ctxKey struct{}

// ContextWithManager stores mgr on ctx so downstream handlers can
// retrieve it via FromContext. Useful when threading request-scoped
// state through middleware.
func ContextWithManager(ctx context.Context, mgr *Manager) context.Context {
	return context.WithValue(ctx, ctxKey{}, mgr)
}

// FromContext returns the Manager stored on ctx, or (nil, false) if
// none has been attached.
func FromContext(ctx context.Context) (*Manager, bool) {
	if ctx == nil {
		return nil, false
	}
	m, ok := ctx.Value(ctxKey{}).(*Manager)
	return m, ok
}

// FromApp returns the Manager published into app by storage.Module.
// Returns (nil, false) when the module is not wired or has not been
// initialised yet.
func FromApp(app *framework.App) (*Manager, bool) {
	v, ok := app.Lookup(StoreKey)
	if !ok {
		return nil, false
	}
	m, ok := v.(*Manager)
	return m, ok
}
