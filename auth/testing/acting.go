// Package testing provides test helpers for auth (Laravel actingAs parity).
package testing

import (
	"context"
	"net/http"

	"github.com/godx-jp/godx-platform-framework/auth"
)

// ActingAs attaches principal to request context for guard without HTTP credentials.
func ActingAs(r *http.Request, guard string, p *auth.Principal) *http.Request {
	if r == nil {
		return r
	}
	if p != nil {
		cp := *p
		cp.Guard = guard
		p = &cp
	}
	ctx := auth.ContextWithPrincipal(r.Context(), p)
	if mgr, ok := auth.FromContext(ctx); ok {
		ctx = auth.ContextWithManager(ctx, mgr)
	} else {
		ctx = auth.ContextWithManager(ctx, auth.NewManager())
	}
	return r.WithContext(ctx)
}

// ActingAsContext injects principal into context (non-HTTP tests).
func ActingAsContext(ctx context.Context, guard string, p *auth.Principal) context.Context {
	if p != nil {
		cp := *p
		cp.Guard = guard
		p = &cp
	}
	ctx = auth.ContextWithPrincipal(ctx, p)
	return auth.ContextWithPrincipalForGuard(ctx, guard, p)
}
