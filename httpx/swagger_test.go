package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestMountOpenAPI_servesDocsAndSpec(t *testing.T) {
	spec := []byte("openapi: 3.1.0\ninfo:\n  title: test\n  version: 1.0.0\npaths: {}\n")
	r := chi.NewRouter()
	MountOpenAPI(r, OpenAPIConfig{Title: "Test Service", Spec: spec})

	t.Run("redirect /docs", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/docs", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusMovedPermanently {
			t.Fatalf("status=%d want 301", rec.Code)
		}
	})

	t.Run("spec yaml", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/docs/openapi.yaml", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "application/yaml; charset=utf-8" {
			t.Fatalf("content-type=%q", ct)
		}
	})

	t.Run("swagger ui", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/docs/", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
			t.Fatalf("content-type=%q", ct)
		}
	})
}

func TestMountOpenAPI_jsonSpecPath(t *testing.T) {
	spec := []byte(`{"swagger":"2.0","info":{"title":"x","version":"1"},"paths":{}}`)
	r := chi.NewRouter()
	MountOpenAPI(r, OpenAPIConfig{Spec: spec})

	req := httptest.NewRequest(http.MethodGet, "/docs/openapi.json", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}
