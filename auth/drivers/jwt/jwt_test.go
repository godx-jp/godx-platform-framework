package jwt

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	adriver "github.com/godx-jp/godx-platform-framework/auth/driver"
	jwtlib "github.com/golang-jwt/jwt/v5"
)

func TestAuthenticateValidJWT(t *testing.T) {
	priv, jwksURL := newTestJWKS(t)
	g, err := adriver.New(context.Background(), adriver.Spec{
		Name:     adriver.DriverJWT,
		JWKSURL:  jwksURL,
		Issuer:   "test-issuer",
		Audience: "test-aud",
	})
	if err != nil {
		t.Fatal(err)
	}

	token := signTestToken(t, priv, jwtlib.MapClaims{
		"sub":         "user-99",
		"iss":         "test-issuer",
		"aud":         "test-aud",
		"roles":       []any{"admin", "user"},
		"permissions": []any{"read", "write"},
	})

	p, err := g.Authenticate(context.Background(), &adriver.CredentialRequest{Token: token})
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if p.SubjectID != "user-99" {
		t.Fatalf("subject=%q", p.SubjectID)
	}
	if len(p.Roles) != 2 || p.Roles[0] != "admin" {
		t.Fatalf("roles=%v", p.Roles)
	}
	if len(p.Permissions) != 2 {
		t.Fatalf("permissions=%v", p.Permissions)
	}
}

func TestAuthenticateWrongIssuer(t *testing.T) {
	priv, jwksURL := newTestJWKS(t)
	g, err := adriver.New(context.Background(), adriver.Spec{
		Name:    adriver.DriverJWT,
		JWKSURL: jwksURL,
		Issuer:  "expected-issuer",
	})
	if err != nil {
		t.Fatal(err)
	}
	token := signTestToken(t, priv, jwtlib.MapClaims{
		"sub": "u1",
		"iss": "wrong-issuer",
	})
	_, err = g.Authenticate(context.Background(), &adriver.CredentialRequest{Token: token})
	if !errors.Is(err, adriver.ErrInvalidCredential) {
		t.Fatalf("err=%v", err)
	}
}

func TestAuthenticateMissingJWKSURL(t *testing.T) {
	_, err := adriver.New(context.Background(), adriver.Spec{Name: adriver.DriverJWT})
	if err == nil {
		t.Fatalf("expected error for missing JWKSURL")
	}
}

func TestJWKSRefreshOnKidMiss(t *testing.T) {
	priv, srv := newRotatingJWKSServer(t)
	g, err := adriver.New(context.Background(), adriver.Spec{
		Name:    adriver.DriverJWT,
		JWKSURL: srv.URL,
	})
	if err != nil {
		t.Fatal(err)
	}

	token1 := signTestTokenWithKid(t, priv, "kid-1", jwtlib.MapClaims{"sub": "u1"})
	if _, err := g.Authenticate(context.Background(), &adriver.CredentialRequest{Token: token1}); err != nil {
		t.Fatalf("first token: %v", err)
	}

	priv2, _ := rsa.GenerateKey(rand.Reader, 2048)
	srv.rotateKey("kid-2", priv2)
	token2 := signTestTokenWithKid(t, priv2, "kid-2", jwtlib.MapClaims{"sub": "u2"})
	p, err := g.Authenticate(context.Background(), &adriver.CredentialRequest{Token: token2})
	if err != nil || p.SubjectID != "u2" {
		t.Fatalf("rotated kid: p=%+v err=%v", p, err)
	}
}

func TestShutdown(t *testing.T) {
	_, jwksURL := newTestJWKS(t)
	g, err := adriver.New(context.Background(), adriver.Spec{
		Name:    adriver.DriverJWT,
		JWKSURL: jwksURL,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := g.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	_, err = g.Authenticate(context.Background(), &adriver.CredentialRequest{Token: "x"})
	if !errors.Is(err, adriver.ErrClosed) {
		t.Fatalf("err=%v", err)
	}
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

type rotatingServer struct {
	*httptest.Server
	mu   sync.RWMutex
	keys []jwkKey
}

func newRotatingJWKSServer(t *testing.T) (*rsa.PrivateKey, *rotatingServer) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	rs := &rotatingServer{keys: []jwkKey{rsaPublicJWK("kid-1", &priv.PublicKey)}}
	rs.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rs.mu.RLock()
		defer rs.mu.RUnlock()
		_ = json.NewEncoder(w).Encode(jwksDocument{Keys: rs.keys})
	}))
	t.Cleanup(rs.Close)
	return priv, rs
}

func (rs *rotatingServer) rotateKey(kid string, priv *rsa.PrivateKey) {
	rs.mu.Lock()
	rs.keys = append(rs.keys, rsaPublicJWK(kid, &priv.PublicKey))
	rs.mu.Unlock()
}

func rsaPublicJWK(kid string, pub *rsa.PublicKey) jwkKey {
	n := base64.RawURLEncoding.EncodeToString(pub.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes())
	return jwkKey{Kty: "RSA", Kid: kid, N: n, E: e}
}

func signTestToken(t *testing.T, priv *rsa.PrivateKey, claims jwtlib.MapClaims) string {
	return signTestTokenWithKid(t, priv, "kid-1", claims)
}

func signTestTokenWithKid(t *testing.T, priv *rsa.PrivateKey, kid string, claims jwtlib.MapClaims) string {
	t.Helper()
	if _, ok := claims["exp"]; !ok {
		claims["exp"] = time.Now().Add(time.Hour).Unix()
	}
	token := jwtlib.NewWithClaims(jwtlib.SigningMethodRS256, claims)
	token.Header["kid"] = kid
	s, err := token.SignedString(priv)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
