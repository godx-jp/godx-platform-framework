// Package middleware provides HTTP authentication middleware for the auth module.
package middleware

import (
	"encoding/json"
	"net/http"

	"github.com/godx-jp/godx-platform-framework/auth"
)

type jsonError struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

func writeJSONError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(jsonError{Error: code, Message: message})
}

func writeUnauthorized(w http.ResponseWriter, message string) {
	if message == "" {
		message = http.StatusText(http.StatusUnauthorized)
	}
	writeJSONError(w, http.StatusUnauthorized, "unauthenticated", message)
}

func writeForbidden(w http.ResponseWriter, message string) {
	if message == "" {
		message = http.StatusText(http.StatusForbidden)
	}
	writeJSONError(w, http.StatusForbidden, "forbidden", message)
}

func effectiveGuardName(mgr *auth.Manager, explicit string) string {
	if explicit != "" {
		return explicit
	}
	return mgr.DefaultName()
}

func resolveForGuard(mgr *auth.Manager, name string) (auth.CredentialResolver, error) {
	g, err := mgr.Guard(name)
	if err != nil {
		return nil, err
	}
	return auth.ResolverForGuard(g.Name(), "X-API-Key")
}

// Authenticate resolves credentials and attaches the Principal to context.
func Authenticate(mgr *auth.Manager, guardName ...string) func(http.Handler) http.Handler {
	guard := ""
	if len(guardName) > 0 {
		guard = guardName[0]
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			name := effectiveGuardName(mgr, guard)
			if name == "" {
				writeUnauthorized(w, "no default guard configured")
				return
			}
			resolve, err := resolveForGuard(mgr, name)
			if err != nil {
				writeUnauthorized(w, err.Error())
				return
			}
			cred, err := resolve(r)
			if err != nil {
				writeUnauthorized(w, err.Error())
				return
			}
			cred.Guard = name
			p, err := mgr.Authenticate(r.Context(), cred)
			if err != nil {
				writeUnauthorized(w, err.Error())
				return
			}
			ctx := auth.ContextWithPrincipal(r.Context(), p)
			ctx = auth.ContextWithManager(ctx, mgr)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Optional runs authentication but continues without a Principal when it fails.
func Optional(mgr *auth.Manager, guardName ...string) func(http.Handler) http.Handler {
	guard := ""
	if len(guardName) > 0 {
		guard = guardName[0]
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := auth.ContextWithManager(r.Context(), mgr)
			name := effectiveGuardName(mgr, guard)
			if name != "" {
				if resolve, err := resolveForGuard(mgr, name); err == nil {
					if cred, err := resolve(r); err == nil {
						cred.Guard = name
						if p, err := mgr.Authenticate(r.Context(), cred); err == nil && p != nil {
							ctx = auth.ContextWithPrincipal(ctx, p)
						}
					}
				}
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole returns 403 when the context Principal lacks any of roles.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, ok := auth.PrincipalFromContext(r.Context())
			if !ok || p == nil {
				writeForbidden(w, "authentication required")
				return
			}
			if !auth.HasRole(p, roles...) {
				writeForbidden(w, "insufficient role")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequirePermission returns 403 when the context Principal lacks any of perms.
func RequirePermission(perms ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, ok := auth.PrincipalFromContext(r.Context())
			if !ok || p == nil {
				writeForbidden(w, "authentication required")
				return
			}
			if !auth.HasPermission(p, perms...) {
				writeForbidden(w, "insufficient permission")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireActorKind returns 403 when the context Principal kind does not match.
func RequireActorKind(kinds ...auth.ActorKind) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, ok := auth.PrincipalFromContext(r.Context())
			if !ok || p == nil {
				writeForbidden(w, "authentication required")
				return
			}
			if !auth.HasActorKind(p, kinds...) {
				writeForbidden(w, "actor kind not allowed")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireGate returns 403 when auth.Check(name, principal) fails.
func RequireGate(name string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, ok := auth.PrincipalFromContext(r.Context())
			if !ok || p == nil {
				writeForbidden(w, "authentication required")
				return
			}
			if !auth.Check(name, p) {
				writeForbidden(w, "gate denied")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
