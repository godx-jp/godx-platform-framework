package httpx_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/godx-jp/godx-platform-framework/httpx"
)

type ctxKey struct{}

func TestTitle(t *testing.T) {
	cases := []struct {
		name string
		slug string
		want string
	}{
		{name: "known slug", slug: "validation-failed", want: "Validation failed"},
		{name: "unknown slug fallback", slug: "unknown-slug", want: "unknown-slug"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := httpx.Title(tc.slug); got != tc.want {
				t.Fatalf("Title(%q) = %q, want %q", tc.slug, got, tc.want)
			}
		})
	}
}

func TestTypeURI_withBaseURL(t *testing.T) {
	httpx.SetProblemTypeBaseURL("https://errors.example.com")
	t.Cleanup(func() { httpx.SetProblemTypeBaseURL("") })

	got := httpx.TypeURI("orders", "tenancy-missing")
	want := "https://errors.example.com/orders/tenancy-missing"
	if got != want {
		t.Fatalf("TypeURI() = %q, want %q", got, want)
	}
}

func TestTypeURI_withoutBaseURL(t *testing.T) {
	httpx.SetProblemTypeBaseURL("")
	got := httpx.TypeURI("orders", "not-found")
	if got != "urn:problem:orders:not-found" {
		t.Fatalf("TypeURI() = %q", got)
	}
}

func TestWriteProblem_StatusMapping(t *testing.T) {
	httpx.SetProblemTypeBaseURL("https://errors.example.com")
	t.Cleanup(func() { httpx.SetProblemTypeBaseURL("") })

	cases := []struct {
		name      string
		slug      string
		detail    string
		status    int
		wantTitle string
	}{
		{name: "bad request", slug: "bad-request", detail: "payload invalid", status: http.StatusBadRequest, wantTitle: "Bad request"},
		{name: "unauthenticated", slug: "unauthenticated", detail: "login required", status: http.StatusUnauthorized, wantTitle: "Unauthenticated"},
		{name: "forbidden", slug: "forbidden", detail: "scope denied", status: http.StatusForbidden, wantTitle: "Forbidden"},
		{name: "not found", slug: "not-found", detail: "missing row", status: http.StatusNotFound, wantTitle: "Not found"},
		{name: "conflict", slug: "conflict", detail: "version conflict", status: http.StatusConflict, wantTitle: "Conflict"},
		{name: "validation", slug: "validation-failed", detail: "field X invalid", status: http.StatusUnprocessableEntity, wantTitle: "Validation failed"},
		{name: "rate limit", slug: "rate-limit", detail: "too many requests", status: http.StatusTooManyRequests, wantTitle: "Too many requests"},
		{name: "internal", slug: "internal-error", detail: "unexpected error", status: http.StatusInternalServerError, wantTitle: "Internal server error"},
		{name: "unavailable", slug: "unavailable", detail: "try later", status: http.StatusServiceUnavailable, wantTitle: "Service unavailable"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/x", nil)

			httpx.WriteProblem(rec, req, "orders", tc.status, tc.slug, tc.detail)

			if got := rec.Code; got != tc.status {
				t.Fatalf("status = %d, want %d", got, tc.status)
			}
			if got := rec.Header().Get("Content-Type"); got != httpx.ContentType {
				t.Fatalf("content-type = %q, want %q", got, httpx.ContentType)
			}
			var p httpx.Problem
			if err := json.NewDecoder(rec.Body).Decode(&p); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if p.Type != httpx.TypeURI("orders", tc.slug) {
				t.Errorf("type = %q", p.Type)
			}
			if p.Title != tc.wantTitle {
				t.Errorf("title = %q, want %q", p.Title, tc.wantTitle)
			}
			if p.Status != tc.status {
				t.Errorf("status = %d, want %d", p.Status, tc.status)
			}
			if p.Code != tc.slug {
				t.Errorf("code = %q, want %q", p.Code, tc.slug)
			}
			if p.Detail != tc.detail {
				t.Errorf("detail = %q, want %q", p.Detail, tc.detail)
			}
		})
	}
}

func TestWriteProblem_Options(t *testing.T) {
	httpx.SetProblemTypeBaseURL("https://errors.example.com")
	t.Cleanup(func() { httpx.SetProblemTypeBaseURL("") })

	t.Run("WithErrors", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		errs := []httpx.FieldError{{Field: "name", Code: "required", Message: "name is required"}}

		httpx.WriteProblem(rec, req, "orders", http.StatusUnprocessableEntity, "validation-failed", "invalid", httpx.WithErrors(errs...))

		var p httpx.Problem
		if err := json.NewDecoder(rec.Body).Decode(&p); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(p.Errors) != 1 {
			t.Fatalf("errors len = %d, want 1", len(p.Errors))
		}
		if p.Errors[0] != errs[0] {
			t.Fatalf("error = %#v, want %#v", p.Errors[0], errs[0])
		}
	})

	t.Run("WithCode", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/x", nil)

		httpx.WriteProblem(rec, req, "orders", http.StatusConflict, "conflict", "conflict", httpx.WithCode("foo"))

		var p httpx.Problem
		if err := json.NewDecoder(rec.Body).Decode(&p); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if p.Code != "foo" {
			t.Fatalf("code = %q, want %q", p.Code, "foo")
		}
	})

	t.Run("WithInstance", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/x", nil)

		httpx.WriteProblem(rec, req, "orders", http.StatusConflict, "conflict", "conflict", httpx.WithInstance("urn:request:abc"))

		var p httpx.Problem
		if err := json.NewDecoder(rec.Body).Decode(&p); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if p.Instance != "urn:request:abc" {
			t.Fatalf("instance = %q, want %q", p.Instance, "urn:request:abc")
		}
	})
}

func TestRequestIDExtractor(t *testing.T) {
	httpx.SetRequestIDExtractor(func(ctx context.Context) string {
		v, _ := ctx.Value(ctxKey{}).(string)
		return v
	})
	t.Cleanup(func() { httpx.SetRequestIDExtractor(func(context.Context) string { return "" }) })
	req := httptest.NewRequest(http.MethodGet, "/x", nil).
		WithContext(context.WithValue(context.Background(), ctxKey{}, "req-abc"))
	rec := httptest.NewRecorder()

	httpx.WriteProblem(rec, req, "orders", http.StatusInternalServerError, "internal-error", "boom")

	var p httpx.Problem
	if err := json.NewDecoder(rec.Body).Decode(&p); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if p.RequestID != "req-abc" {
		t.Errorf("request_id = %q, want %q", p.RequestID, "req-abc")
	}
	if p.Instance != "req-abc" {
		t.Errorf("instance = %q, want %q (default to request_id)", p.Instance, "req-abc")
	}
}
