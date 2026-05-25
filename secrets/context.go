package secrets

import (
	"context"
	"fmt"

	"github.com/godx-jp/godx-platform-framework/framework"
)

type contextKey struct{}

// ContextWithManager returns a derived context carrying mgr.
func ContextWithManager(ctx context.Context, mgr *Manager) context.Context {
	return context.WithValue(ctx, contextKey{}, mgr)
}

// FromContext retrieves the Manager attached to ctx. ok == false
// when no manager is present.
func FromContext(ctx context.Context) (*Manager, bool) {
	if ctx == nil {
		return nil, false
	}
	mgr, ok := ctx.Value(contextKey{}).(*Manager)
	return mgr, ok
}

// FromApp returns the Manager published by secrets.Module on app.
func FromApp(app *framework.App) (*Manager, error) {
	v, ok := app.Lookup(StoreKey)
	if !ok {
		return nil, fmt.Errorf("secrets: Module has not been initialised on this App")
	}
	mgr, ok := v.(*Manager)
	if !ok {
		return nil, fmt.Errorf("secrets: %s framework Store entry is not a *Manager (%T)", StoreKey, v)
	}
	return mgr, nil
}
