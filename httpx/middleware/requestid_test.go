package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/godx-jp/godx-platform-framework/httpx/middleware"
)

func TestRequestID_generatesAndEchoes(t *testing.T) {
	var got string
	h := middleware.RequestID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = middleware.RequestIDFrom(r.Context())
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeHTTP(rec, req)
	if got == "" {
		t.Fatal("expected request id in context")
	}
	if rec.Header().Get(middleware.RequestIDHeader) != got {
		t.Fatalf("header=%q ctx=%q", rec.Header().Get(middleware.RequestIDHeader), got)
	}
}

func TestRequestID_propagatesIncoming(t *testing.T) {
	const want = "req-existing"
	h := middleware.RequestID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if middleware.RequestIDFrom(r.Context()) != want {
			t.Fatalf("context id mismatch")
		}
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(middleware.RequestIDHeader, want)
	h.ServeHTTP(rec, req)
	if rec.Header().Get(middleware.RequestIDHeader) != want {
		t.Fatalf("header=%q", rec.Header().Get(middleware.RequestIDHeader))
	}
}

func TestRecover_catchesPanic(t *testing.T) {
	h := middleware.Recover()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}
