package httpx

import (
	"errors"
	"net/http"
	"net/http/httptest"
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

func TestJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	JSON(rec, http.StatusOK, map[string]string{"ok": "true"})
	if rec.Code != 200 {
		t.Fatalf("code=%d", rec.Code)
	}
}
