package observability_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/godx-jp/godx-platform-framework/observability"
)

func TestMiddleware_PropagatesCorrelationAndTrace(t *testing.T) {
	_ = captureStdout(t, func() {
		p, err := observability.NewProvider(context.Background(), observability.Config{
			ServiceName: "mw-svc",
			Driver:      observability.DriverStdout,
			LogLevel:    slog.LevelInfo,
		})
		if err != nil {
			t.Fatalf("NewProvider: %v", err)
		}
		defer p.Shutdown(context.Background())

		var (
			gotCID   string
			gotInner *observability.Provider
		)

		handler := p.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotCID = observability.CorrelationIDFromContext(r.Context())
			gotInner = observability.FromContext(r.Context())
			w.WriteHeader(http.StatusTeapot)
		}))

		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		req.Header.Set(observability.CorrelationHeader, "cid-fixed")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusTeapot {
			t.Errorf("status = %d, want 418", rec.Code)
		}
		if rec.Header().Get(observability.CorrelationHeader) != "cid-fixed" {
			t.Errorf("response header CID = %q, want cid-fixed", rec.Header().Get(observability.CorrelationHeader))
		}
		if gotCID != "cid-fixed" {
			t.Errorf("downstream CID = %q, want cid-fixed", gotCID)
		}
		if gotInner != p {
			t.Errorf("provider not propagated through context")
		}
	})
}

func TestMiddleware_GeneratesCorrelationWhenMissing(t *testing.T) {
	_ = captureStdout(t, func() {
		p, err := observability.NewProvider(context.Background(), observability.Config{
			ServiceName: "mw-svc",
			Driver:      observability.DriverStdout,
		})
		if err != nil {
			t.Fatalf("NewProvider: %v", err)
		}
		defer p.Shutdown(context.Background())

		var gotCID string
		handler := p.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotCID = observability.CorrelationIDFromContext(r.Context())
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if len(gotCID) != 32 {
			t.Errorf("generated CID len = %d, want 32 hex chars", len(gotCID))
		}
		if rec.Header().Get(observability.CorrelationHeader) != gotCID {
			t.Errorf("response CID header does not match downstream CID")
		}
	})
}
