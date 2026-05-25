package health

import (
	"context"
	"fmt"

	"github.com/godx-jp/godx-platform-framework/framework"
)

func FromApp(app *framework.App) (*Registry, error) {
	v, ok := app.Lookup(StoreKey)
	if !ok {
		return nil, fmt.Errorf("health: Module has not been initialised on this App")
	}
	reg, ok := v.(*Registry)
	if !ok {
		return nil, fmt.Errorf("health: %s is not a *Registry (%T)", StoreKey, v)
	}
	return reg, nil
}

func ContextWithRegistry(ctx context.Context, reg *Registry) context.Context {
	return context.WithValue(ctx, contextKey{}, reg)
}

func FromContext(ctx context.Context) (*Registry, bool) {
	if ctx == nil {
		return nil, false
	}
	reg, ok := ctx.Value(contextKey{}).(*Registry)
	return reg, ok
}

type contextKey struct{}
