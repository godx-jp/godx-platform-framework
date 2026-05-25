package auth

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
	"testing"
	"time"

	adriver "github.com/godx-jp/godx-platform-framework/auth/driver"
	_ "github.com/godx-jp/godx-platform-framework/auth/drivers/introspect"
	jwtlib "github.com/golang-jwt/jwt/v5"
)

type guardCase struct {
	name        string
	build       func(t *testing.T) adriver.Guard
	valid       *adriver.CredentialRequest
	invalid     *adriver.CredentialRequest
	validErr    error
	invalidIs   error
	skipInvalid bool
}

func runGuardConformance(t *testing.T, gc guardCase) {
	t.Helper()
	t.Run(gc.name, func(t *testing.T) {
		t.Run("name_matches_driver", func(t *testing.T) {
			g := gc.build(t)
			defer g.Shutdown(context.Background())
			if g.Name() != gc.name {
				t.Fatalf("Name=%q want %q", g.Name(), gc.name)
			}
		})

		t.Run("authenticate_valid", func(t *testing.T) {
			g := gc.build(t)
			defer g.Shutdown(context.Background())
			p, err := g.Authenticate(context.Background(), gc.valid)
			if gc.validErr != nil {
				if !errors.Is(err, gc.validErr) {
					t.Fatalf("Authenticate err=%v want %v", err, gc.validErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Authenticate: %v", err)
			}
			if p == nil || p.SubjectID == "" {
				t.Fatalf("principal=%+v", p)
			}
		})

		if !gc.skipInvalid {
			t.Run("authenticate_invalid", func(t *testing.T) {
				g := gc.build(t)
				defer g.Shutdown(context.Background())
				_, err := g.Authenticate(context.Background(), gc.invalid)
				want := gc.invalidIs
				if want == nil {
					want = adriver.ErrInvalidCredential
				}
				if !errors.Is(err, want) {
					t.Fatalf("Authenticate err=%v want %v", err, want)
				}
			})
		}

		t.Run("shutdown_is_idempotent_and_blocks_ops", func(t *testing.T) {
			g := gc.build(t)
			if err := g.Shutdown(context.Background()); err != nil {
				t.Fatalf("first Shutdown: %v", err)
			}
			if err := g.Shutdown(context.Background()); err != nil {
				t.Fatalf("second Shutdown: %v", err)
			}
			_, err := g.Authenticate(context.Background(), gc.valid)
			if !errors.Is(err, adriver.ErrClosed) {
				t.Fatalf("Authenticate after Shutdown err=%v", err)
			}
		})
	})
}

func apikeyCase() guardCase {
	return guardCase{
		name: adriver.DriverAPIKey,
		build: func(t *testing.T) adriver.Guard {
			t.Helper()
			g, err := adriver.New(context.Background(), adriver.Spec{
				Name: adriver.DriverAPIKey,
				Keys: map[string]adriver.APIKeyEntry{
					"svc": {SubjectID: "svc-1", Secret: "top-secret", ActorKind: adriver.ActorService},
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			return g
		},
		valid:   &adriver.CredentialRequest{APIKey: "top-secret"},
		invalid: &adriver.CredentialRequest{APIKey: "wrong"},
	}
}

func jwtCase(t *testing.T) guardCase {
	t.Helper()
	priv, jwksURL := newConformanceJWKS(t)
	token := signConformanceJWT(t, priv, jwtlib.MapClaims{"sub": "user-conform"})
	return guardCase{
		name: adriver.DriverJWT,
		build: func(t *testing.T) adriver.Guard {
			t.Helper()
			g, err := adriver.New(context.Background(), adriver.Spec{
				Name:    adriver.DriverJWT,
				JWKSURL: jwksURL,
			})
			if err != nil {
				t.Fatal(err)
			}
			return g
		},
		valid:   &adriver.CredentialRequest{Token: token},
		invalid: &adriver.CredentialRequest{Token: "not-a-jwt"},
	}
}

func introspectCase() guardCase {
	return guardCase{
		name: adriver.DriverIntrospect,
		build: func(t *testing.T) adriver.Guard {
			t.Helper()
			g, err := adriver.New(context.Background(), adriver.Spec{
				Name:          adriver.DriverIntrospect,
				IntrospectURL: "https://idp.example.com/introspect",
			})
			if err != nil {
				t.Fatal(err)
			}
			return g
		},
		valid:       &adriver.CredentialRequest{Token: "opaque-token"},
		validErr:    adriver.ErrNotImplemented,
		skipInvalid: true,
	}
}

func TestConformance(t *testing.T) {
	runGuardConformance(t, apikeyCase())
	runGuardConformance(t, jwtCase(t))
	runGuardConformance(t, introspectCase())
}

func newConformanceJWKS(t *testing.T) (*rsa.PrivateKey, string) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := base64.RawURLEncoding.EncodeToString(priv.N.Bytes())
		e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(priv.E)).Bytes())
		_ = json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]string{
				{"kty": "RSA", "kid": "kid-1", "n": n, "e": e},
			},
		})
	}))
	t.Cleanup(srv.Close)
	return priv, srv.URL
}

func signConformanceJWT(t *testing.T, priv *rsa.PrivateKey, claims jwtlib.MapClaims) string {
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
