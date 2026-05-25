package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

// RequestIDHeader is the de-facto request correlation HTTP header.
const RequestIDHeader = "X-Request-ID"

type requestIDKey struct{}

// RequestID propagates an incoming RequestIDHeader or generates a UUID.
// The ID is echoed on the response for client correlation.
func RequestID() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get(RequestIDHeader)
			if id == "" {
				if v7, err := uuid.NewV7(); err == nil {
					id = v7.String()
				} else {
					id = uuid.NewString()
				}
			}
			w.Header().Set(RequestIDHeader, id)
			ctx := context.WithValue(r.Context(), requestIDKey{}, id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequestIDFrom returns the request ID stored by RequestID middleware.
func RequestIDFrom(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey{}).(string); ok {
		return v
	}
	return ""
}
