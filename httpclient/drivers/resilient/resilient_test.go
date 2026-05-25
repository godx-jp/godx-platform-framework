package resilient_test

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	hdriver "github.com/godx-jp/godx-platform-framework/httpclient/driver"
	_ "github.com/godx-jp/godx-platform-framework/httpclient/drivers/resilient"
)

func TestRegisteredOnImport(t *testing.T) {
	if hdriver.Lookup(hdriver.DriverResilient) == nil {
		t.Fatal("resilient driver not registered")
	}
}

// newClient builds a resilient client targeting srv with maxRetries
// extra attempts (so total attempts == maxRetries+1).
func newClient(t *testing.T, srv *httptest.Server, maxRetries int) hdriver.Client {
	t.Helper()
	c, err := hdriver.New(context.Background(), hdriver.Spec{
		Name:         hdriver.DriverResilient,
		BaseURL:      srv.URL,
		MaxRetries:   maxRetries,
		RetryBackoff: time.Millisecond,
		// Keep the circuit breaker out of the way of the retry test.
		CBMaxFailures: 1000,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func do(t *testing.T, c hdriver.Client, method string) {
	t.Helper()
	req, err := http.NewRequest(method, "/", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	resp, _ := c.Do(context.Background(), req)
	if resp != nil {
		resp.Body.Close()
	}
}

func Test5xxRetriedForSafeMethods(t *testing.T) {
	const maxRetries = 3 // → up to 4 attempts
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodOptions} {
		method := method
		t.Run(method, func(t *testing.T) {
			var calls int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				atomic.AddInt32(&calls, 1)
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer srv.Close()

			c := newClient(t, srv, maxRetries)
			do(t, c, method)

			if got := atomic.LoadInt32(&calls); got != maxRetries+1 {
				t.Fatalf("%s on 5xx: got %d attempts, want %d", method, got, maxRetries+1)
			}
		})
	}
}

func Test5xxNotRetriedForUnsafeMethods(t *testing.T) {
	const maxRetries = 3
	for _, method := range []string{http.MethodDelete, http.MethodPut, http.MethodPost, http.MethodPatch} {
		method := method
		t.Run(method, func(t *testing.T) {
			var calls int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				atomic.AddInt32(&calls, 1)
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer srv.Close()

			c := newClient(t, srv, maxRetries)
			do(t, c, method)

			if got := atomic.LoadInt32(&calls); got != 1 {
				t.Fatalf("%s on 5xx: got %d attempts, want exactly 1 (no retry)", method, got)
			}
		})
	}
}

// syncBuffer is a concurrency-safe io.Writer for capturing slog output;
// the resilient driver may emit from a retry goroutine.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

// TestCircuitBreakerLogsOpen verifies the resilient driver's default
// OnStateChange logs a Warn-level "circuit breaker opened" line when the
// breaker trips, closing RFC 0001 G4 (the silent breaker).
func TestCircuitBreakerLogsOpen(t *testing.T) {
	var sink syncBuffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&sink, &slog.HandlerOptions{Level: slog.LevelInfo})))
	defer slog.SetDefault(prev)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	// MaxFailures=2, no retries: each safe GET that 5xxs records one
	// failure, so two GETs trip the breaker.
	c, err := hdriver.New(context.Background(), hdriver.Spec{
		Name:           hdriver.DriverResilient,
		BaseURL:        srv.URL,
		MaxRetries:     0,
		RetryBackoff:   time.Millisecond,
		CBMaxFailures:  2,
		CBResetTimeout: time.Hour,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	do(t, c, http.MethodGet)
	do(t, c, http.MethodGet)

	out := sink.String()
	if !strings.Contains(out, "circuit breaker opened") {
		t.Fatalf("missing open log line; got:\n%s", out)
	}
	if !strings.Contains(out, "level=WARN") {
		t.Fatalf("open should log at WARN; got:\n%s", out)
	}
	if !strings.Contains(out, "to=open") || !strings.Contains(out, "target="+srv.URL) {
		t.Fatalf("expected to=open and target attr; got:\n%s", out)
	}
}

func Test2xxNotRetried(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newClient(t, srv, 3)
	do(t, c, http.MethodGet)

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("2xx GET: got %d attempts, want 1", got)
	}
}
