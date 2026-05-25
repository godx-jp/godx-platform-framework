package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/godx-jp/godx-platform-framework/auth"
	adriver "github.com/godx-jp/godx-platform-framework/auth/driver"
	authmw "github.com/godx-jp/godx-platform-framework/auth/middleware"
)

func TestAuthenticateUnknownGuardReturns401(t *testing.T) {
	mgr := auth.NewManager()
	_ = mgr.AddGuard("jwt", staticGuard{driver: adriver.DriverJWT, id: &adriver.Principal{SubjectID: "u"}})
	_ = mgr.SetDefault("jwt")

	h := authmw.Authenticate(mgr, "missing")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("should not reach handler")
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer x")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAuthenticateNoDefaultGuardReturns401(t *testing.T) {
	mgr := auth.NewManager()
	h := authmw.Authenticate(mgr)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("should not reach handler")
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestRequireRoleWithoutPrincipalReturns403(t *testing.T) {
	h := authmw.RequireRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("should not reach handler")
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestRequirePermissionWithoutPrincipalReturns403(t *testing.T) {
	h := authmw.RequirePermission("read")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("should not reach handler")
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestRequireActorKindWithoutPrincipalReturns403(t *testing.T) {
	h := authmw.RequireActorKind(auth.ActorHuman)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("should not reach handler")
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestRequireGateWithoutPrincipalReturns403(t *testing.T) {
	h := authmw.RequireGate("any")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("should not reach handler")
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestOptionalWithBadCredentialsStillOK(t *testing.T) {
	mgr := testManager(t, nil, adriver.ErrInvalidCredential)
	called := false
	h := authmw.Optional(mgr)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if _, ok := auth.PrincipalFromContext(r.Context()); ok {
			t.Fatal("should not have principal")
		}
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if !called || rec.Code != http.StatusOK {
		t.Fatalf("called=%v status=%d", called, rec.Code)
	}
}

func TestRequireRoleEmptyRolesListDenied(t *testing.T) {
	mgr := testManager(t, &adriver.Principal{SubjectID: "u1", Roles: []string{"admin"}}, nil)
	h := authmw.Authenticate(mgr)(authmw.RequireRole()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("should not reach handler")
	})))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer tok")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d", rec.Code)
	}
}
