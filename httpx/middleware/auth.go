package middleware

import (
	"net/http"

	"github.com/godx-jp/godx-platform-framework/auth"
	authmw "github.com/godx-jp/godx-platform-framework/auth/middleware"
)

// Authenticate wraps auth/middleware.Authenticate for httpx stacks.
func Authenticate(mgr *auth.Manager, guardName ...string) func(http.Handler) http.Handler {
	return authmw.Authenticate(mgr, guardName...)
}

// Optional wraps auth/middleware.Optional for httpx stacks.
func Optional(mgr *auth.Manager, guardName ...string) func(http.Handler) http.Handler {
	return authmw.Optional(mgr, guardName...)
}

// RequireRole wraps auth/middleware.RequireRole for httpx stacks.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	return authmw.RequireRole(roles...)
}

// RequirePermission wraps auth/middleware.RequirePermission for httpx stacks.
func RequirePermission(perms ...string) func(http.Handler) http.Handler {
	return authmw.RequirePermission(perms...)
}

// RequireActorKind wraps auth/middleware.RequireActorKind for httpx stacks.
func RequireActorKind(kinds ...auth.ActorKind) func(http.Handler) http.Handler {
	return authmw.RequireActorKind(kinds...)
}

// RequireGate wraps auth/middleware.RequireGate for httpx stacks.
func RequireGate(name string) func(http.Handler) http.Handler {
	return authmw.RequireGate(name)
}
