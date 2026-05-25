package auth

import (
	"context"
	"fmt"

	"github.com/godx-jp/godx-platform-framework/framework"
)

type contextKey struct{}
type guardMapKey struct{}

func ContextWithPrincipal(ctx context.Context, p *Principal) context.Context {
	ctx = context.WithValue(ctx, contextKey{}, p)
	if p != nil && p.Guard != "" {
		ctx = ContextWithPrincipalForGuard(ctx, p.Guard, p)
	}
	return ctx
}

// ContextWithPrincipalForGuard stores a principal for a named guard.
func ContextWithPrincipalForGuard(ctx context.Context, guard string, p *Principal) context.Context {
	if ctx == nil || guard == "" {
		return ctx
	}
	m, _ := ctx.Value(guardMapKey{}).(map[string]*Principal)
	if m == nil {
		m = map[string]*Principal{}
	}
	m[guard] = p
	return context.WithValue(ctx, guardMapKey{}, m)
}

// PrincipalForGuard returns the principal bound to guard name.
func PrincipalForGuard(ctx context.Context, guard string) (*Principal, bool) {
	if ctx == nil || guard == "" {
		return nil, false
	}
	m, ok := ctx.Value(guardMapKey{}).(map[string]*Principal)
	if !ok {
		return nil, false
	}
	p, ok := m[guard]
	return p, ok && p != nil
}

func PrincipalFromContext(ctx context.Context) (*Principal, bool) {
	if ctx == nil {
		return nil, false
	}
	p, ok := ctx.Value(contextKey{}).(*Principal)
	return p, ok
}

func SubjectIDFromContext(ctx context.Context) (string, bool) {
	p, ok := PrincipalFromContext(ctx)
	if !ok || p == nil || p.SubjectID == "" {
		return "", false
	}
	return p.SubjectID, true
}

// UserIDFromContext is an alias for SubjectIDFromContext.
func UserIDFromContext(ctx context.Context) (string, bool) {
	return SubjectIDFromContext(ctx)
}

func ContextWithManager(ctx context.Context, mgr *Manager) context.Context {
	return context.WithValue(ctx, managerKey{}, mgr)
}

func FromContext(ctx context.Context) (*Manager, bool) {
	if ctx == nil {
		return nil, false
	}
	mgr, ok := ctx.Value(managerKey{}).(*Manager)
	return mgr, ok
}

type managerKey struct{}

func FromApp(app *framework.App) (*Manager, error) {
	v, ok := app.Lookup(StoreKey)
	if !ok {
		return nil, fmt.Errorf("auth: Module not initialised")
	}
	mgr, ok := v.(*Manager)
	if !ok {
		return nil, fmt.Errorf("auth: invalid store entry %T", v)
	}
	return mgr, nil
}
