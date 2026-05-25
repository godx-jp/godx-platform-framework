package ratelimit

import (
	"context"
	"fmt"

	"github.com/godx-jp/godx-platform-framework/framework"
)

type contextKey struct{}

func ContextWithManager(ctx context.Context, mgr *Manager) context.Context {
	return context.WithValue(ctx, contextKey{}, mgr)
}

func FromContext(ctx context.Context) (*Manager, bool) {
	if ctx == nil {
		return nil, false
	}
	mgr, ok := ctx.Value(contextKey{}).(*Manager)
	return mgr, ok
}

func FromApp(app *framework.App) (*Manager, error) {
	v, ok := app.Lookup(StoreKey)
	if !ok {
		return nil, fmt.Errorf("ratelimit: Module not initialised")
	}
	mgr, ok := v.(*Manager)
	if !ok {
		return nil, fmt.Errorf("ratelimit: invalid store entry %T", v)
	}
	return mgr, nil
}
