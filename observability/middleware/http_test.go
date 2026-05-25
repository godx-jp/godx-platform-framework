package middleware_test

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"

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

// TestHTTP_LogSeverityByStatus verifies the per-request log level reflects the
// response status: 5xx→ERROR, 4xx→WARN, else INFO.
func TestHTTP_LogSeverityByStatus(t *testing.T) {
	cases := []struct {
		status    int
		wantLevel string
	}{
		{http.StatusOK, "INFO"},
		{http.StatusNotFound, "WARN"},
		{http.StatusInternalServerError, "ERROR"},
	}
	for _, tc := range cases {
		out := captureStdout(t, func() {
			p, err := observability.NewProvider(context.Background(), observability.Config{
				ServiceName: "sev-svc",
				Driver:      observability.DriverStdout,
				LogLevel:    slog.LevelInfo,
			})
			if err != nil {
				t.Fatalf("NewProvider: %v", err)
			}
			defer p.Shutdown(context.Background())

			handler := middleware.HTTP(p)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
			}))
			handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/x", nil))
		})
		if want := `"level":"` + tc.wantLevel + `"`; !strings.Contains(out, want) {
			t.Errorf("status %d: log missing %s\n%s", tc.status, want, out)
		}
	}
}

// TestHTTP_RouteTemplateInLog verifies the middleware records the low-cardinality
// chi route template (not the concrete path) as http.route.
func TestHTTP_RouteTemplateInLog(t *testing.T) {
	out := captureStdout(t, func() {
		p, err := observability.NewProvider(context.Background(), observability.Config{
			ServiceName: "route-svc",
			Driver:      observability.DriverStdout,
			LogLevel:    slog.LevelInfo,
		})
		if err != nil {
			t.Fatalf("NewProvider: %v", err)
		}
		defer p.Shutdown(context.Background())

		r := chi.NewRouter()
		r.Use(middleware.HTTP(p))
		r.Get("/users/{id}", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
		r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/users/42", nil))
	})
	if !strings.Contains(out, `"http.route":"/users/{id}"`) {
		t.Errorf("log missing route template\n%s", out)
	}
	if !strings.Contains(out, `"path":"/users/42"`) {
		t.Errorf("log missing concrete path\n%s", out)
	}
}

// TestHTTP_UnmatchedRoute verifies requests not served through a chi router fall
// back to the constant route label, bounding cardinality.
func TestHTTP_UnmatchedRoute(t *testing.T) {
	out := captureStdout(t, func() {
		p, err := observability.NewProvider(context.Background(), observability.Config{
			ServiceName: "unmatched-svc",
			Driver:      observability.DriverStdout,
			LogLevel:    slog.LevelInfo,
		})
		if err != nil {
			t.Fatalf("NewProvider: %v", err)
		}
		defer p.Shutdown(context.Background())

		handler := middleware.HTTP(p)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/anything/123", nil))
	})
	if !strings.Contains(out, `"http.route":"<unmatched>"`) {
		t.Errorf("log missing unmatched route fallback\n%s", out)
	}
}
