package middleware

import (
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/godx-jp/godx-platform-framework/observability"
)

// InstrumentedRouter returns a chi.Mux pre-wired with the correct observability
// defaults: request-id, the observability HTTP middleware (tracing + RED metrics
// + severity-mapped logging from Phase 1's HTTP()), and panic recovery. A single
// correct entry point so services don't hand-wire the stack. chi RealIP is NOT
// enabled (spoofable; opt in behind a trusted proxy — see the v1.8 ratelimit fix).
//
// Middleware order is outermost-first: RequestID → HTTP(p) → Recoverer. HTTP(p)
// wraps Recoverer so a recovered panic's 500 status is observed (logged at
// ERROR, counted, and recorded on the span) rather than escaping the
// instrumentation.
func InstrumentedRouter(p *observability.Provider) *chi.Mux {
	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(HTTP(p))
	r.Use(chimiddleware.Recoverer)
	return r
}
