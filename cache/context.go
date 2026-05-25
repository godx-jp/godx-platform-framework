package cache

import (
	"context"
	"fmt"

	"github.com/godx-jp/godx-platform-framework/framework"
)

type contextKey struct{}

// ContextWithManager returns a derived context carrying mgr. Handlers
// that prefer pulling the manager from context.Context over a closure
// can use this together with FromContext.
func ContextWithManager(ctx context.Context, mgr *Manager) context.Context {
	return context.WithValue(ctx, contextKey{}, mgr)
}

// FromContext retrieves the Manager attached to ctx by
// ContextWithManager. ok == false when no manager is present.
func FromContext(ctx context.Context) (*Manager, bool) {
	if ctx == nil {
		return nil, false
	}
	mgr, ok := ctx.Value(contextKey{}).(*Manager)
	return mgr, ok
}

// FromApp is the canonical way to retrieve the manager built by
// cache.Module from a framework.App.
func FromApp(app *framework.App) (*Manager, error) {
	v, ok := app.Lookup(StoreKey)
	if !ok {
		return nil, fmt.Errorf("cache: Module has not been initialised on this App (did you call app.Use(cache.Module).Init(ctx)?)")
	}
	mgr, ok := v.(*Manager)
	if !ok {
		return nil, fmt.Errorf("cache: %s framework Store entry is not a *Manager (%T)", StoreKey, v)
	}
	return mgr, nil
}
