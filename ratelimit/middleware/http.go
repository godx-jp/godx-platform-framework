// Package middleware provides HTTP middleware for the ratelimit module.
package middleware

import (
	"fmt"
	"math"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"time"

	rdriver "github.com/godx-jp/godx-platform-framework/ratelimit/driver"
)

// KeyFunc extracts the rate-limit key from a request.
type KeyFunc func(*http.Request) string

// ByIP rate-limits by the connection peer (r.RemoteAddr).
//
// It deliberately ignores X-Forwarded-For and X-Real-IP: those headers are
// client-controlled and trivially spoofed, which would let an attacker mint
// unlimited fresh rate-limit buckets. For deployments behind a trusted
// reverse proxy or load balancer, use [ByForwardedFor] instead.
func ByIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// ByForwardedFor returns a KeyFunc that honours X-Forwarded-For only when the
// connection peer (r.RemoteAddr) is itself within one of trustedProxies. It
// then walks the XFF chain from right to left and returns the right-most hop
// that is NOT a trusted proxy — i.e. the real client as seen by your edge.
//
// When the peer is not a trusted proxy, XFF is ignored entirely and the
// behaviour falls back to [ByIP] (the peer address). This makes header
// spoofing impossible from untrusted sources.
//
// SECURITY: never trust raw X-Forwarded-For at an internet-facing edge.
// trustedProxies must list only the CIDRs of proxies/load balancers you
// operate; an empty list disables XFF parsing entirely.
func ByForwardedFor(trustedProxies []netip.Prefix) KeyFunc {
	return func(r *http.Request) string {
		peer := ByIP(r)
		peerAddr, err := netip.ParseAddr(peer)
		if err != nil || !inTrustedProxies(peerAddr, trustedProxies) {
			return peer
		}
		xff := r.Header.Get("X-Forwarded-For")
		if xff == "" {
			return peer
		}
		hops := strings.Split(xff, ",")
		for i := len(hops) - 1; i >= 0; i-- {
			hop := strings.TrimSpace(hops[i])
			addr, err := netip.ParseAddr(hop)
			if err != nil {
				continue
			}
			if !inTrustedProxies(addr, trustedProxies) {
				return addr.String()
			}
		}
		// Every hop was a trusted proxy; fall back to the peer.
		return peer
	}
}

func inTrustedProxies(addr netip.Addr, prefixes []netip.Prefix) bool {
	if !addr.IsValid() {
		return false
	}
	addr = addr.Unmap()
	for _, p := range prefixes {
		if p.Contains(addr) {
			return true
		}
	}
	return false
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
