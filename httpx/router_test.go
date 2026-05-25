package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewRouterAndRoute(t *testing.T) {
	r := NewRouter()
	Route(r, http.MethodGet, "/ping", func(w http.ResponseWriter, r *http.Request) error {
		JSON(w, http.StatusOK, map[string]string{"pong": "1"})
		return nil
	})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ping", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}
