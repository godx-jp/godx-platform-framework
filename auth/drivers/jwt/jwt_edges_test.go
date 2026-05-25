package jwt

import (
	"context"
	"errors"
	"testing"
	"time"

	adriver "github.com/godx-jp/godx-platform-framework/auth/driver"
	jwtlib "github.com/golang-jwt/jwt/v5"
)

func TestAuthenticateWrongAudience(t *testing.T) {
	priv, jwksURL := newTestJWKS(t)
	g, err := adriver.New(context.Background(), adriver.Spec{
		Name:     adriver.DriverJWT,
		JWKSURL:  jwksURL,
		Audience: "expected-aud",
	})
	if err != nil {
		t.Fatal(err)
	}
	token := signTestToken(t, priv, jwtlib.MapClaims{
		"sub": "u1",
		"aud": "wrong-aud",
	})
	_, err = g.Authenticate(context.Background(), &adriver.CredentialRequest{Token: token})
	if !errors.Is(err, adriver.ErrInvalidCredential) {
		t.Fatalf("err=%v", err)
	}
}

func TestAuthenticateMissingSubject(t *testing.T) {
	priv, jwksURL := newTestJWKS(t)
	g, err := adriver.New(context.Background(), adriver.Spec{
		Name:    adriver.DriverJWT,
		JWKSURL: jwksURL,
	})
	if err != nil {
		t.Fatal(err)
	}
	token := signTestToken(t, priv, jwtlib.MapClaims{"roles": []string{"admin"}})
	_, err = g.Authenticate(context.Background(), &adriver.CredentialRequest{Token: token})
	if !errors.Is(err, adriver.ErrInvalidCredential) {
		t.Fatalf("err=%v", err)
	}
}

func TestAuthenticateExpiredToken(t *testing.T) {
	priv, jwksURL := newTestJWKS(t)
	g, err := adriver.New(context.Background(), adriver.Spec{
		Name:    adriver.DriverJWT,
		JWKSURL: jwksURL,
	})
	if err != nil {
		t.Fatal(err)
	}
	token := signTestToken(t, priv, jwtlib.MapClaims{
		"sub": "u1",
		"exp": time.Now().Add(-time.Hour).Unix(),
	})
	_, err = g.Authenticate(context.Background(), &adriver.CredentialRequest{Token: token})
	if !errors.Is(err, adriver.ErrInvalidCredential) {
		t.Fatalf("err=%v", err)
	}
}

func TestAuthenticateEmptyToken(t *testing.T) {
	_, jwksURL := newTestJWKS(t)
	g, err := adriver.New(context.Background(), adriver.Spec{
		Name:    adriver.DriverJWT,
		JWKSURL: jwksURL,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = g.Authenticate(context.Background(), &adriver.CredentialRequest{Token: ""})
	if !errors.Is(err, adriver.ErrInvalidCredential) {
		t.Fatalf("err=%v", err)
	}
}

func TestAuthenticateCustomClaims(t *testing.T) {
	priv, jwksURL := newTestJWKS(t)
	g, err := adriver.New(context.Background(), adriver.Spec{
		Name:           adriver.DriverJWT,
		JWKSURL:        jwksURL,
		RolesClaim:     "realm_roles",
		SubjectClaim:   "user_id",
		ActorKindClaim: "kind",
	})
	if err != nil {
		t.Fatal(err)
	}
	token := signTestToken(t, priv, jwtlib.MapClaims{
		"user_id":     "custom-sub",
		"kind":        "service",
		"realm_roles": []string{"ops"},
	})
	p, err := g.Authenticate(context.Background(), &adriver.CredentialRequest{Token: token})
	if err != nil {
		t.Fatal(err)
	}
	if p.SubjectID != "custom-sub" || p.ActorKind != adriver.ActorService || p.Roles[0] != "ops" {
		t.Fatalf("p=%+v", p)
	}
}

func TestAuthenticateRolesAsStringClaim(t *testing.T) {
	priv, jwksURL := newTestJWKS(t)
	g, _ := adriver.New(context.Background(), adriver.Spec{Name: adriver.DriverJWT, JWKSURL: jwksURL})
	token := signTestToken(t, priv, jwtlib.MapClaims{"sub": "u1", "roles": "single-role"})
	p, err := g.Authenticate(context.Background(), &adriver.CredentialRequest{Token: token})
	if err != nil || len(p.Roles) != 1 || p.Roles[0] != "single-role" {
		t.Fatalf("p=%+v err=%v", p, err)
	}
}
