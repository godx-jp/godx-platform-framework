package resilient_test

import (
	"context"
	"net/http"
	"net/http/httptest"
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
