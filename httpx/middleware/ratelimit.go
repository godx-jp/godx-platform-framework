package middleware

import (
	"net/http"

	rdriver "github.com/godx-jp/godx-platform-framework/ratelimit/driver"
	rlmw "github.com/godx-jp/godx-platform-framework/ratelimit/middleware"
)

// RateLimit returns middleware that rate-limits by keyFunc using limiter l.
func RateLimit(l rdriver.Limiter, keyFunc rlmw.KeyFunc) func(http.Handler) http.Handler {
	if keyFunc == nil {
		keyFunc = rlmw.ByIP
	}
	return rlmw.Limit(l, keyFunc)
}

// RateLimitByIP rate-limits requests by client IP.
func RateLimitByIP(l rdriver.Limiter) func(http.Handler) http.Handler {
	return rlmw.Limit(l, rlmw.ByIP)
}
