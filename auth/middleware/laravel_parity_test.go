package middleware_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/godx-jp/godx-platform-framework/auth"
	adriver "github.com/godx-jp/godx-platform-framework/auth/driver"
	_ "github.com/godx-jp/godx-platform-framework/auth/drivers/apikey"
	_ "github.com/godx-jp/godx-platform-framework/auth/drivers/jwt"
	authmw "github.com/godx-jp/godx-platform-framework/auth/middleware"
	jwtlib "github.com/golang-jwt/jwt/v5"
)

// Laravel HTTP / Sanctum parity tests.
// References: laravel/docs http-tests (assertGuest), sanctum ability middleware (401 vs 403).

func buildRealManager(t *testing.T, cfg auth.Config) *auth.Manager {
	t.Helper()
	ctx := context.Background()
	mgr := auth.NewManager()
	for name, gc := range cfg.Guards {
		spec := gc.Spec
		if spec.Name == "" {
			spec.Name = gc.Driver
		}
		g, err := adriver.New(ctx, spec)
		if err != nil {
			t.Fatal(err)
		}
		if err := mgr.AddGuard(name, g); err != nil {
			t.Fatal(err)
		}
		resolve, err := auth.ResolverForGuard(gc.Driver, spec.Header)
		if err != nil {
			t.Fatal(err)
		}
		if err := mgr.SetResolver(name, resolve); err != nil {
			t.Fatal(err)
		}
	}
	if err := mgr.SetDefault(cfg.Default); err != nil {
		t.Fatal(err)
	}
	return mgr
}

func TestLaravelGuestGets401Not403(t *testing.T) {
	mgr := testManager(t, nil, adriver.ErrInvalidCredential)
	h := authmw.Authenticate(mgr)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("guest must not reach handler")
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("Laravel assertGuest: want 401 got %d", rec.Code)
	}
	var body struct{ Error string }
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body.Error == "forbidden" {
		t.Fatal("guest must be unauthenticated (401), not forbidden (403)")
	}
}

func TestLaravelAuthenticatedButUnauthorizedGets403Not401(t *testing.T) {
	mgr := testManager(t, &adriver.Principal{SubjectID: "u1", Roles: []string{"viewer"}}, nil)
	h := authmw.Authenticate(mgr)(authmw.RequireRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("should not reach handler")
	})))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer tok")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("Laravel policy deny: want 403 got %d", rec.Code)
	}
}

func TestLaravelSanctumAbilityAnyMatch(t *testing.T) {
	// Sanctum ability middleware: at least one ability (RequirePermission uses ANY)
	mgr := testManager(t, &adriver.Principal{
		SubjectID:   "u1",
		Permissions: []string{"read"},
	}, nil)
	h := authmw.Authenticate(mgr)(authmw.RequirePermission("write", "read")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer tok")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 when one ability matches, got %d", rec.Code)
	}
}

func TestLaravelSanctumAbilityNoneMatch403(t *testing.T) {
	mgr := testManager(t, &adriver.Principal{
		SubjectID:   "u1",
		Permissions: []string{"read"},
	}, nil)
	h := authmw.Authenticate(mgr)(authmw.RequirePermission("write", "delete")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("should not reach handler")
	})))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer tok")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when no abilities match, got %d", rec.Code)
	}
}

func TestLaravelWrongGuardDoesNotCrossAuthenticate(t *testing.T) {
	// Laravel custom-guard issue: credentials for guard A must not satisfy guard B route
	priv, jwksURL := newTestJWKS(t)
	mgr := buildRealManager(t, auth.Config{
		Default: "jwt",
		Guards: map[string]auth.GuardConfig{
			"jwt": {Driver: adriver.DriverJWT, Spec: adriver.Spec{Name: adriver.DriverJWT, JWKSURL: jwksURL}},
			"apikey": {
				Driver: adriver.DriverAPIKey,
				Spec: adriver.Spec{
					Name: adriver.DriverAPIKey,
					Keys: map[string]adriver.APIKeyEntry{
						"s": {SubjectID: "svc", Secret: "sekret"},
					},
				},
			},
		},
	})
	token := signTestToken(t, priv, jwtlib.MapClaims{"sub": "jwt-user"})

	// JWT token on apikey guard route → must 401 (apikey resolver ignores Bearer)
	h := authmw.Authenticate(mgr, "apikey")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("wrong guard must not authenticate")
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong guard want 401 got %d", rec.Code)
	}
}

func TestLaravelCustomAPIKeyHeaderViaModuleResolver(t *testing.T) {
	mgr := buildRealManager(t, auth.Config{
		Default: "api",
		Guards: map[string]auth.GuardConfig{
			"api": {
				Driver: adriver.DriverAPIKey,
				Spec: adriver.Spec{
					Name:   adriver.DriverAPIKey,
					Header: "X-Custom-Api-Key",
					Keys: map[string]adriver.APIKeyEntry{
						"s": {SubjectID: "svc", Secret: "sekret"},
					},
				},
			},
		},
	})
	h := authmw.Authenticate(mgr, "api")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := auth.PrincipalFromContext(r.Context())
		if !ok || p.SubjectID != "svc" {
			t.Fatalf("principal=%+v ok=%v", p, ok)
		}
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Custom-Api-Key", "sekret")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("custom header auth failed: %d body=%s", rec.Code, rec.Body.String())
	}

	// Default X-API-Key must NOT work when custom header configured
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-API-Key", "sekret")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong header must 401, got %d", rec.Code)
	}
}

func TestLaravelMiddlewareStackAuthBeforeCan(t *testing.T) {
	gate := "laravel-stack-gate-" + t.Name()
	auth.MustDefine(gate, func(p *auth.Principal) bool {
		return auth.HasRole(p, "admin")
	})
	mgr := testManager(t, &adriver.Principal{SubjectID: "u1", Roles: []string{"admin"}}, nil)
	h := authmw.Authenticate(mgr)(authmw.RequireGate(gate)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer tok")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("auth then gate should pass, got %d", rec.Code)
	}
}

func TestLaravelOptionalEnrichesAuthenticatedOnly(t *testing.T) {
	mgr := testManager(t, &adriver.Principal{SubjectID: "u1"}, nil)
	h := authmw.Optional(mgr)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := auth.PrincipalFromContext(r.Context()); !ok {
			t.Fatal("optional with valid token should enrich context")
		}
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer tok")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}