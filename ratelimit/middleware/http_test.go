package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
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

type brokenLimiter struct{}

func (brokenLimiter) Name() string { return "broken" }
func (brokenLimiter) Allow(context.Context, string) (bool, error) {
	return false, rdriver.ErrClosed
}
func (brokenLimiter) Reset(context.Context, string)  {}
func (brokenLimiter) Shutdown(context.Context) error { return nil }
