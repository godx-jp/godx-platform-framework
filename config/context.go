package config

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

// FromContext retrieves the Manager attached to ctx by
// ContextWithManager. ok == false when no manager is present.
func FromContext(ctx context.Context) (*Manager, bool) {
	if ctx == nil {
		return nil, false
	}
	mgr, ok := ctx.Value(contextKey{}).(*Manager)
	return mgr, ok
}

// FromApp returns the Repository owned by the Manager that
// config.Module installed on app.
func FromApp(app *framework.App) (*Repository, error) {
	mgr, err := ManagerFromApp(app)
	if err != nil {
		return nil, err
	}
	return mgr.Repository(), nil
}

// ManagerFromApp returns the *Manager directly. Most code wants
// FromApp; reach for ManagerFromApp when you need to Reload or
// register a new source after Init.
func ManagerFromApp(app *framework.App) (*Manager, error) {
	v, ok := app.Lookup(StoreKey)
	if !ok {
		return nil, fmt.Errorf("config: Module has not been initialised on this App (did you call app.Use(config.Module).Init(ctx)?)")
	}
	mgr, ok := v.(*Manager)
	if !ok {
		return nil, fmt.Errorf("config: %s framework Store entry is not a *Manager (%T)", StoreKey, v)
	}
	return mgr, nil
}
