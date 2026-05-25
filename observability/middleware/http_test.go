package middleware_test

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/godx-jp/godx-platform-framework/observability"
	"github.com/godx-jp/godx-platform-framework/observability/middleware"
)

// captureStdout is duplicated from the observability package's test helpers —
// kept here so middleware tests stay decoupled from the parent package's
// _test.go internals.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	var buf bytes.Buffer
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = buf.ReadFrom(r)
	}()

	fn()

	_ = w.Close()
	wg.Wait()
	os.Stdout = orig
	return buf.String()
}

func TestHTTP_PropagatesCorrelationAndTrace(t *testing.T) {
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

		wrap := middleware.HTTP(p)
		handler := wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotCID = observability.CorrelationIDFromContext(r.Context())
			gotInner = observability.FromContext(r.Context())
			w.WriteHeader(http.StatusTeapot)
		}))

		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		req.Header.Set(middleware.CorrelationHeader, "cid-fixed")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusTeapot {
			t.Errorf("status = %d, want 418", rec.Code)
		}
		if rec.Header().Get(middleware.CorrelationHeader) != "cid-fixed" {
			t.Errorf("response header CID = %q, want cid-fixed", rec.Header().Get(middleware.CorrelationHeader))
		}
		if gotCID != "cid-fixed" {
			t.Errorf("downstream CID = %q, want cid-fixed", gotCID)
		}
		if gotInner != p {
			t.Errorf("provider not propagated through context")
		}
	})
}

func TestHTTP_GeneratesCorrelationWhenMissing(t *testing.T) {
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
		wrap := middleware.HTTP(p)
		handler := wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotCID = observability.CorrelationIDFromContext(r.Context())
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if len(gotCID) != 32 {
			t.Errorf("generated CID len = %d, want 32 hex chars", len(gotCID))
		}
		if rec.Header().Get(middleware.CorrelationHeader) != gotCID {
			t.Errorf("response CID header does not match downstream CID")
		}
	})
}
