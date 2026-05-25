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
	// RealIP enables chi RealIP middleware.
	RealIP bool
	// Recoverer enables chi panic recoverer.
	Recoverer bool
}

// NewRouter returns a chi.Mux with sensible defaults for API services.
func NewRouter(opts ...RouterOptions) *chi.Mux {
	var o RouterOptions
	if len(opts) > 0 {
		o = opts[0]
	}
	r := chi.NewRouter()
	if o.RequestID || (!o.RequestID && !o.RealIP && !o.Recoverer) {
		r.Use(middleware.RequestID)
	}
	if o.RealIP || (!o.RequestID && !o.RealIP && !o.Recoverer) {
		r.Use(middleware.RealIP)
	}
	if o.Recoverer || (!o.RequestID && !o.RealIP && !o.Recoverer) {
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
