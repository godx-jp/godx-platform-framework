package httpx

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// RouterOptions tunes the default chi router.
type RouterOptions struct {
	// RequestID enables chi request ID middleware.
	RequestID bool
	// RealIP enables chi RealIP middleware, which rewrites r.RemoteAddr from
	// the X-Forwarded-For / X-Real-IP request headers.
	//
	// SECURITY: those headers are client-controlled and trivially spoofed, so
	// RealIP is NOT enabled by default. Enable it only when the service sits
	// behind a trusted reverse proxy / load balancer that overwrites them;
	// enabling it at an internet-facing edge lets clients forge their own IP
	// and defeats any RemoteAddr-based rate limiting.
	RealIP bool
	// Recoverer enables chi panic recoverer.
	Recoverer bool
}

// NewRouter returns a chi.Mux with sensible defaults for API services.
//
// When called with no options the defaults are RequestID + Recoverer only.
// RealIP is opt-in (see [RouterOptions.RealIP]) and is never part of the
// default set.
func NewRouter(opts ...RouterOptions) *chi.Mux {
	var o RouterOptions
	if len(opts) > 0 {
		o = opts[0]
	} else {
		// Safe-by-default set: identify requests and recover from panics,
		// but never trust client IP headers.
		o = RouterOptions{RequestID: true, Recoverer: true}
	}
	r := chi.NewRouter()
	if o.RequestID {
		r.Use(middleware.RequestID)
	}
	if o.RealIP {
		r.Use(middleware.RealIP)
	}
	if o.Recoverer {
		r.Use(middleware.Recoverer)
	}
	return r
}

// Route registers method+pattern with a HandlerFunc on r.
func Route(r chi.Router, method, pattern string, h HandlerFunc) {
	r.Method(method, pattern, Serve(h))
}

// Group applies middleware to a sub-router built by fn.
func Group(r chi.Router, fn func(r chi.Router), stages ...func(http.Handler) http.Handler) {
	r.Group(func(sub chi.Router) {
		for _, s := range stages {
			sub.Use(s)
		}
		fn(sub)
	})
}
