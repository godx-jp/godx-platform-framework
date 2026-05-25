package httpclient

import (
	"context"
	"fmt"

	"github.com/godx-jp/godx-platform-framework/framework"
)

type contextKey struct{}

func ContextWithManager(ctx context.Context, m *Manager) context.Context {
	return context.WithValue(ctx, contextKey{}, m)
}

func FromContext(ctx context.Context) (*Manager, bool) {
	if ctx == nil {
		return nil, false
	}
	m, ok := ctx.Value(contextKey{}).(*Manager)
	return m, ok
}

func FromApp(app *framework.App) (*Manager, error) {
	v, ok := app.Lookup(StoreKey)
	if !ok {
		return nil, fmt.Errorf("httpclient: Module not initialised")
	}
	m, ok := v.(*Manager)
	if !ok {
		return nil, fmt.Errorf("httpclient: invalid store entry %T", v)
	}
	return m, nil
}
