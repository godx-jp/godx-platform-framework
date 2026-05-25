package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	rdriver "github.com/godx-jp/godx-platform-framework/ratelimit/driver"
	memdrv "github.com/godx-jp/godx-platform-framework/ratelimit/drivers/memory"
	"github.com/godx-jp/godx-platform-framework/ratelimit/middleware"
)

func TestMiddlewareAllowsThenBlocks(t *testing.T) {
	lim := memdrv.New(100, 1)
	handler := middleware.Limit(lim, middleware.ByIP)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.1:1234"

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("first status=%d", rec.Code)
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second status=%d want 429", rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got == "" {
		t.Fatalf("missing Retry-After header")
	}
}

func TestMiddlewareByHeader(t *testing.T) {
	lim := memdrv.New(100, 1)
	keyFunc := middleware.ByHeader("X-API-Key")
	handler := middleware.Limit(lim, keyFunc)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	reqA := httptest.NewRequest(http.MethodGet, "/", nil)
	reqA.Header.Set("X-API-Key", "alpha")
	reqB := httptest.NewRequest(http.MethodGet, "/", nil)
	reqB.Header.Set("X-API-Key", "beta")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, reqA)
	if rec.Code != http.StatusOK {
		t.Fatalf("alpha first: %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, reqA)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("alpha second: %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, reqB)
	if rec.Code != http.StatusOK {
		t.Fatalf("beta should have separate bucket: %d", rec.Code)
	}
}

func TestMiddlewareLimiterError(t *testing.T) {
	lim := &brokenLimiter{}
	handler := middleware.Limit(lim, middleware.ByIP)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestByIPIgnoresXForwardedFor(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.7:5555"
	req.Header.Set("X-Forwarded-For", "10.0.0.1, 8.8.8.8")
	if got := middleware.ByIP(req); got != "203.0.113.7" {
		t.Fatalf("ByIP=%q want connection peer host 203.0.113.7 (XFF must be ignored)", got)
	}
}

func TestByIPNoXFFReturnsPeer(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "198.51.100.4:9090"
	if got := middleware.ByIP(req); got != "198.51.100.4" {
		t.Fatalf("ByIP=%q want 198.51.100.4", got)
	}
}

func TestByForwardedForHonorsXFFFromTrustedProxy(t *testing.T) {
	trusted := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
	kf := middleware.ByForwardedFor(trusted)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.1.2.3:443" // peer is a trusted proxy
	req.Header.Set("X-Forwarded-For", "203.0.113.9, 10.9.9.9")
	// Right-most untrusted hop is the real client: 203.0.113.9.
	if got := kf(req); got != "203.0.113.9" {
		t.Fatalf("ByForwardedFor=%q want 203.0.113.9", got)
	}
}

func TestByForwardedForIgnoresXFFFromUntrustedPeer(t *testing.T) {
	trusted := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
	kf := middleware.ByForwardedFor(trusted)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.50:1111" // peer NOT a trusted proxy
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	// Untrusted peer => XFF ignored, fall back to peer host.
	if got := kf(req); got != "203.0.113.50" {
		t.Fatalf("ByForwardedFor=%q want peer 203.0.113.50 (spoofed XFF must be ignored)", got)
	}
}

func TestByForwardedForAllHopsTrustedFallsBackToPeer(t *testing.T) {
	trusted := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
	kf := middleware.ByForwardedFor(trusted)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.1.2.3:443"
	req.Header.Set("X-Forwarded-For", "10.9.9.9, 10.8.8.8")
	if got := kf(req); got != "10.1.2.3" {
		t.Fatalf("ByForwardedFor=%q want peer 10.1.2.3", got)
	}
}

type brokenLimiter struct{}

func (brokenLimiter) Name() string { return "broken" }
func (brokenLimiter) Allow(context.Context, string) (bool, error) {
	return false, rdriver.ErrClosed
}
func (brokenLimiter) Reset(context.Context, string)  {}
func (brokenLimiter) Shutdown(context.Context) error { return nil }
