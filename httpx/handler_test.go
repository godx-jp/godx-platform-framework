package httpx

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServeStatusError(t *testing.T) {
	h := Serve(func(w http.ResponseWriter, r *http.Request) error {
		return NewStatusError(http.StatusNotFound, "missing")
	})
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestServeInternalError(t *testing.T) {
	h := Serve(func(w http.ResponseWriter, r *http.Request) error {
		return errors.New("boom")
	})
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestDecodeJSONValid(t *testing.T) {
	var dst struct {
		Name string `json:"name"`
	}
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"ok"}`))
	if err := DecodeJSON(req, &dst); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst.Name != "ok" {
		t.Fatalf("name=%q", dst.Name)
	}
}

func TestDecodeJSONMalformedReturns400(t *testing.T) {
	var dst struct {
		Name string `json:"name"`
	}
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":`))
	err := DecodeJSON(req, &dst)
	var se *StatusError
	if !errors.As(err, &se) || se.Code != http.StatusBadRequest {
		t.Fatalf("want 400 StatusError, got %v", err)
	}
}

func TestDecodeJSONOversizedReturns413(t *testing.T) {
	var dst struct {
		Name string `json:"name"`
	}
	// Build a body larger than the limit we pass.
	big := `{"name":"` + strings.Repeat("a", 4096) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(big))
	err := DecodeJSONLimit(req, &dst, 128)
	var se *StatusError
	if !errors.As(err, &se) {
		t.Fatalf("want StatusError, got %T %v", err, err)
	}
	if se.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("want 413, got %d", se.Code)
	}
}

func TestDecodeJSONOversizedNotSurfacedAs500(t *testing.T) {
	// A handler using DecodeJSONLimit should map oversize to 413, not 500.
	h := Serve(func(w http.ResponseWriter, r *http.Request) error {
		var dst struct {
			Name string `json:"name"`
		}
		return DecodeJSONLimit(r, &dst, 16)
	})
	big := `{"name":"` + strings.Repeat("a", 1024) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(big))
	rec := httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("want 413, got %d", rec.Code)
	}
}

func TestJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	JSON(rec, http.StatusOK, map[string]string{"ok": "true"})
	if rec.Code != 200 {
		t.Fatalf("code=%d", rec.Code)
	}
}
