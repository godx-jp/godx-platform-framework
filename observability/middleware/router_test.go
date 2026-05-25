package middleware_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/godx-jp/godx-platform-framework/observability"
	"github.com/godx-jp/godx-platform-framework/observability/middleware"
)

// TestInstrumentedRouter_RecoversPanicAs500 verifies that a route which panics
// is caught by the Recoverer middleware (returns 500, no test crash) and that
// HTTP() wraps Recoverer so the recovered 500 is observed at ERROR level.
func TestInstrumentedRouter_RecoversPanicAs500(t *testing.T) {
	out := captureStdout(t, func() {
		p, err := observability.NewProvider(context.Background(), observability.Config{
			ServiceName: "router-svc",
			Driver:      observability.DriverStdout,
			LogLevel:    slog.LevelInfo,
		})
		if err != nil {
			t.Fatalf("NewProvider: %v", err)
		}
		defer p.Shutdown(context.Background())

		r := middleware.InstrumentedRouter(p)
		r.Get("/boom", func(w http.ResponseWriter, _ *http.Request) {
			panic("kaboom")
		})

		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/boom", nil))

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500 (Recoverer should catch the panic)", rec.Code)
		}
	})

	// HTTP() wraps Recoverer, so the recovered 500 is logged at ERROR.
	if !strings.Contains(out, `"level":"ERROR"`) {
		t.Errorf("expected ERROR-level log for recovered 500\n%s", out)
	}
	if !strings.Contains(out, `"status":500`) {
		t.Errorf("expected status 500 in observability log\n%s", out)
	}
}

// TestInstrumentedRouter_NormalRequest verifies a non-panicking route is served
// normally and observed.
func TestInstrumentedRouter_NormalRequest(t *testing.T) {
	out := captureStdout(t, func() {
		p, err := observability.NewProvider(context.Background(), observability.Config{
			ServiceName: "router-svc",
			Driver:      observability.DriverStdout,
			LogLevel:    slog.LevelInfo,
		})
		if err != nil {
			t.Fatalf("NewProvider: %v", err)
		}
		defer p.Shutdown(context.Background())

		r := middleware.InstrumentedRouter(p)
		r.Get("/ok", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ok", nil))

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", rec.Code)
		}
	})

	if !strings.Contains(out, `"http.route":"/ok"`) {
		t.Errorf("expected route template in observability log\n%s", out)
	}
}
