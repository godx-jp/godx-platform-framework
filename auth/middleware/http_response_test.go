package middleware_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/godx-jp/godx-platform-framework/auth"
	adriver "github.com/godx-jp/godx-platform-framework/auth/driver"
	authmw "github.com/godx-jp/godx-platform-framework/auth/middleware"
)

func TestUnauthorizedResponseJSON(t *testing.T) {
	mgr := testManager(t, nil, adriver.ErrInvalidCredential)
	h := authmw.Authenticate(mgr)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("should not reach handler")
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rec.Code)
	}
	var body struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error != "unauthenticated" {
		t.Fatalf("error=%q", body.Error)
	}
}

func TestForbiddenResponseJSON(t *testing.T) {
	mgr := testManager(t, &adriver.Principal{SubjectID: "u1", Roles: []string{"viewer"}}, nil)
	h := authmw.Authenticate(mgr)(authmw.RequireRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("should not reach handler")
	})))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer tok")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d", rec.Code)
	}
	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error != "forbidden" {
		t.Fatalf("error=%q", body.Error)
	}
}

func TestRequireGateAllowsWhenCheckPasses(t *testing.T) {
	gateName := "allow-gate-" + t.Name()
	auth.MustDefine(gateName, func(p *auth.Principal) bool {
		return p != nil && auth.HasRole(p, "admin")
	})
	mgr := testManager(t, &adriver.Principal{SubjectID: "u1", Roles: []string{"admin"}}, nil)
	h := authmw.Authenticate(mgr)(authmw.RequireGate(gateName)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer tok")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
