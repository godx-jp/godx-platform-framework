package database

import (
	"context"
	"fmt"

	"github.com/godx-jp/godx-platform-framework/framework"
)

const StoreKey = "godx.database.manager"

// FromApp returns the Manager published by database.Module.
func FromApp(app *framework.App) (*Manager, error) {
	v, ok := app.Lookup(StoreKey)
	if !ok {
		return nil, fmt.Errorf("database: Module has not been initialised on this App")
	}
	mgr, ok := v.(*Manager)
	if !ok {
		return nil, fmt.Errorf("database: %s is not a *Manager (%T)", StoreKey, v)
	}
	return mgr, nil
}

type contextKey struct{}

// ContextWithManager attaches mgr to ctx.
func ContextWithManager(ctx context.Context, mgr *Manager) context.Context {
	return context.WithValue(ctx, contextKey{}, mgr)
}

// FromContext returns the Manager from ctx.
func FromContext(ctx context.Context) (*Manager, bool) {
	if ctx == nil {
		return nil, false
	}
	mgr, ok := ctx.Value(contextKey{}).(*Manager)
	return mgr, ok
}
