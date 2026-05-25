// Package middleware provides HTTP middleware for the ratelimit module.
package middleware

import (
	"fmt"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	rdriver "github.com/godx-jp/godx-platform-framework/ratelimit/driver"
)

// KeyFunc extracts the rate-limit key from a request.
type KeyFunc func(*http.Request) string

// ByIP rate-limits by client IP (RemoteAddr, honouring X-Forwarded-For).
func ByIP(r *http.Request) string {
	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// ByHeader rate-limits by the first value of header name.
func ByHeader(name string) KeyFunc {
	return func(r *http.Request) string {
		return strings.TrimSpace(r.Header.Get(name))
	}
}

// Options configures HTTP rate-limit middleware.
type Options struct {
	Limiter    rdriver.Limiter
	KeyFunc    KeyFunc
	RetryAfter time.Duration
}

// Handler returns middleware that calls next when Allow succeeds.
// On denial it responds with 429 Too Many Requests and Retry-After.
func Handler(opts Options) func(http.Handler) http.Handler {
	if opts.KeyFunc == nil {
		opts.KeyFunc = ByIP
	}
	if opts.RetryAfter <= 0 {
		opts.RetryAfter = time.Second
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := opts.KeyFunc(r)
			if key == "" {
				key = "_"
			}
			ok, err := opts.Limiter.Allow(r.Context(), key)
			if err != nil {
				http.Error(w, "rate limiter error", http.StatusInternalServerError)
				return
			}
			if !ok {
				sec := int(math.Ceil(opts.RetryAfter.Seconds()))
				if sec < 1 {
					sec = 1
				}
				w.Header().Set("Retry-After", strconv.Itoa(sec))
				http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Limit is a convenience wrapper around Handler.
func Limit(l rdriver.Limiter, keyFunc KeyFunc) func(http.Handler) http.Handler {
	return Handler(Options{Limiter: l, KeyFunc: keyFunc})
}

// LimitWithRetryAfter is Handler with an explicit Retry-After duration.
func LimitWithRetryAfter(l rdriver.Limiter, keyFunc KeyFunc, retryAfter time.Duration) func(http.Handler) http.Handler {
	return Handler(Options{
		Limiter:    l,
		KeyFunc:    keyFunc,
		RetryAfter: retryAfter,
	})
}

// StringKey joins parts into a single limiter key.
func StringKey(parts ...string) string {
	return strings.Join(parts, ":")
}

// UserKey is a helper KeyFunc for authenticated routes.
func UserKey(header string) KeyFunc {
	return func(r *http.Request) string {
		v := strings.TrimSpace(r.Header.Get(header))
		if v == "" {
			return ByIP(r)
		}
		return fmt.Sprintf("user:%s", v)
	}
}
