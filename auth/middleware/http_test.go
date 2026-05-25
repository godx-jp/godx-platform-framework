package middleware_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/godx-jp/godx-platform-framework/auth"
	adriver "github.com/godx-jp/godx-platform-framework/auth/driver"
	_ "github.com/godx-jp/godx-platform-framework/auth/drivers/apikey"
	_ "github.com/godx-jp/godx-platform-framework/auth/drivers/jwt"
	authmw "github.com/godx-jp/godx-platform-framework/auth/middleware"
	jwtlib "github.com/golang-jwt/jwt/v5"
)

type staticGuard struct {
	driver string
	id     *adriver.Principal
	err    error
}

func (g staticGuard) Name() string {
	if g.driver != "" {
		return g.driver
	}
	return adriver.DriverJWT
}
func (g staticGuard) Authenticate(context.Context, *adriver.CredentialRequest) (*adriver.Principal, error) {
	return g.id, g.err
}
func (staticGuard) Shutdown(context.Context) error { return nil }

func testManager(t *testing.T, id *adriver.Principal, err error) *auth.Manager {
	t.Helper()
	mgr := auth.NewManager()
	if err := mgr.AddGuard("default", staticGuard{driver: adriver.DriverJWT, id: id, err: err}); err != nil {
		t.Fatal(err)
	}
	if err := mgr.SetDefault("default"); err != nil {
		t.Fatal(err)
	}
	return mgr
}

func TestAuthenticateSuccess(t *testing.T) {
	mgr := testManager(t, &adriver.Principal{
		SubjectID: "user-1",
		ActorKind: adriver.ActorHuman,
		Roles:     []string{"admin"},
	}, nil)
	h := authmw.Authenticate(mgr)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := auth.PrincipalFromContext(r.Context())
		if !ok || p.SubjectID != "user-1" {
			t.Fatalf("principal=%v ok=%v", p, ok)
		}
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer token")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAuthenticateUnauthorized(t *testing.T) {
	mgr := testManager(t, nil, adriver.ErrInvalidCredential)
	h := authmw.Authenticate(mgr)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("should not reach handler")
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["error"] != "unauthenticated" {
		t.Fatalf("body=%v", body)
	}
}

func TestOptionalContinuesWithoutPrincipal(t *testing.T) {
	mgr := testManager(t, nil, adriver.ErrInvalidCredential)
	called := false
	h := authmw.Optional(mgr)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if _, ok := auth.PrincipalFromContext(r.Context()); ok {
			t.Fatal("expected no principal")
		}
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if !called || rec.Code != http.StatusOK {
		t.Fatalf("called=%v status=%d", called, rec.Code)
	}
}

func TestRequireRoleForbidden(t *testing.T) {
	mgr := testManager(t, &adriver.Principal{SubjectID: "u", Roles: []string{"viewer"}}, nil)
	h := authmw.Authenticate(mgr)(authmw.RequireRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("should not reach handler")
	})))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer t")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestRequirePermissionForbidden(t *testing.T) {
	mgr := testManager(t, &adriver.Principal{SubjectID: "u", Permissions: []string{"read"}}, nil)
	h := authmw.Authenticate(mgr)(authmw.RequirePermission("write")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("should not reach handler")
	})))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer t")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestRequireActorKindForbidden(t *testing.T) {
	mgr := testManager(t, &adriver.Principal{SubjectID: "u", ActorKind: adriver.ActorHuman}, nil)
	h := authmw.Authenticate(mgr)(authmw.RequireActorKind(auth.ActorService)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("should not reach handler")
	})))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer t")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestRequireGateForbidden(t *testing.T) {
	gateName := "can-edit-" + time.Now().Format("150405.000000")
	if err := auth.Define(gateName, func(p *auth.Principal) bool {
		return auth.HasRole(p, "editor")
	}); err != nil {
		t.Fatal(err)
	}
	mgr := testManager(t, &adriver.Principal{SubjectID: "u", Roles: []string{"viewer"}}, nil)
	h := authmw.Authenticate(mgr)(authmw.RequireGate(gateName)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("should not reach handler")
	})))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer t")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestJWTGuardIntegration(t *testing.T) {
	priv, jwksURL := newTestJWKS(t)
	mgr := auth.NewManager()
	g, err := adriver.New(context.Background(), adriver.Spec{
		Name:    adriver.DriverJWT,
		JWKSURL: jwksURL,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := mgr.AddGuard("jwt", g); err != nil {
		t.Fatal(err)
	}
	if err := mgr.SetDefault("jwt"); err != nil {
		t.Fatal(err)
	}
	token := signTestToken(t, priv, jwtlib.MapClaims{
		"sub":   "alice",
		"roles": []string{"admin"},
	})
	h := authmw.Authenticate(mgr)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, _ := auth.PrincipalFromContext(r.Context())
		if p.SubjectID != "alice" || !auth.HasRole(p, "admin") {
			t.Fatalf("principal=%+v", p)
		}
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAPIKeyGuardIntegration(t *testing.T) {
	mgr := auth.NewManager()
	g, err := adriver.New(context.Background(), adriver.Spec{
		Name: adriver.DriverAPIKey,
		Keys: map[string]adriver.APIKeyEntry{
			"svc": {SubjectID: "svc", Secret: "sekret", ActorKind: adriver.ActorService},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := mgr.AddGuard("apikey", g); err != nil {
		t.Fatal(err)
	}
	if err := mgr.SetDefault("apikey"); err != nil {
		t.Fatal(err)
	}
	h := authmw.Authenticate(mgr, "apikey")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, _ := auth.PrincipalFromContext(r.Context())
		if p.ActorKind != auth.ActorService {
			t.Fatalf("kind=%q", p.ActorKind)
		}
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-API-Key", "sekret")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

type jwksDocument struct {
	Keys []jwkKey `json:"keys"`
}

type jwkKey struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
}

func newTestJWKS(t *testing.T) (*rsa.PrivateKey, string) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(jwksDocument{Keys: []jwkKey{rsaPublicJWK("kid-1", &priv.PublicKey)}})
	}))
	t.Cleanup(srv.Close)
	return priv, srv.URL
}

func rsaPublicJWK(kid string, pub *rsa.PublicKey) jwkKey {
	n := base64.RawURLEncoding.EncodeToString(pub.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes())
	return jwkKey{Kty: "RSA", Kid: kid, N: n, E: e}
}

func signTestToken(t *testing.T, priv *rsa.PrivateKey, claims jwtlib.MapClaims) string {
	t.Helper()
	if _, ok := claims["exp"]; !ok {
		claims["exp"] = time.Now().Add(time.Hour).Unix()
	}
	token := jwtlib.NewWithClaims(jwtlib.SigningMethodRS256, claims)
	token.Header["kid"] = "kid-1"
	s, err := token.SignedString(priv)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
