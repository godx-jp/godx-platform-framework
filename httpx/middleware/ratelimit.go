package middleware

import (
	"net/http"
	"time"

	rlmw "github.com/godx-jp/godx-platform-framework/ratelimit/middleware"
	rdriver "github.com/godx-jp/godx-platform-framework/ratelimit/driver"
)

// RateLimit wraps the ratelimit module HTTP middleware for httpx stacks.
func RateLimit(l rdriver.Limiter, keyFunc rlmw.KeyFunc) func(http.Handler) http.Handler {
	return rlmw.Handler(rlmw.Options{
		Limiter:    l,
		KeyFunc:    keyFunc,
		RetryAfter: time.Second,
	})
}

// RateLimitByIP is a convenience wrapper using client IP keys.
func RateLimitByIP(l rdriver.Limiter) func(http.Handler) http.Handler {
	return RateLimit(l, rlmw.ByIP)
}
