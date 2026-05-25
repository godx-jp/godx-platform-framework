// Run: go run ./examples/auth from repo root.
package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/godx-jp/godx-platform-framework/auth"
	adriver "github.com/godx-jp/godx-platform-framework/auth/driver"
	authmw "github.com/godx-jp/godx-platform-framework/auth/middleware"
	"github.com/godx-jp/godx-platform-framework/framework"
	jwtlib "github.com/golang-jwt/jwt/v5"
)

func main() {
	ctx := context.Background()
	priv, jwksURL := newTestJWKS()
	cfg := auth.Config{
		Default: "jwt",
		Guards: map[string]auth.GuardConfig{
			"jwt": {
				Driver: adriver.DriverJWT,
				Spec: adriver.Spec{
					Name:    adriver.DriverJWT,
					JWKSURL: jwksURL,
				},
			},
			"apikey": {
				Driver: adriver.DriverAPIKey,
				Spec: adriver.Spec{
					Name: adriver.DriverAPIKey,
					Keys: map[string]adriver.APIKeyEntry{
						"internal": {
							SubjectID: "internal-svc",
							Secret:    "service-key-123",
							ActorKind: adriver.ActorService,
							Roles:     []string{"api"},
						},
					},
				},
			},
		},
	}
	app := framework.New("auth-example", "0.0.0").Use(auth.ModuleWithConfig(cfg))
	if err := app.Init(ctx); err != nil {
		log.Fatalf("init: %v", err)
	}
	defer app.Shutdown(ctx)

	mgr, _ := auth.FromApp(app)
	if err := auth.Define("manage-posts", func(p *auth.Principal) bool {
		return auth.HasRole(p, "admin") || auth.HasPermission(p, "posts:manage")
	}); err != nil {
		log.Fatalf("define gate: %v", err)
	}

	r := chi.NewRouter()
	r.Get("/public", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, "public ok")
	})
	r.Group(func(r chi.Router) {
		r.Use(authmw.Authenticate(mgr, "jwt"))
		r.Get("/me", func(w http.ResponseWriter, r *http.Request) {
			p, _ := auth.PrincipalFromContext(r.Context())
			fmt.Fprintf(w, "jwt user=%s roles=%v\n", p.SubjectID, p.Roles)
		})
		r.With(authmw.RequireGate("manage-posts")).Get("/posts/manage", func(w http.ResponseWriter, _ *http.Request) {
			fmt.Fprintln(w, "posts managed")
		})
	})
	r.Group(func(r chi.Router) {
		r.Use(authmw.Authenticate(mgr, "apikey"))
		r.With(authmw.RequireActorKind(auth.ActorService)).Get("/internal/ping", func(w http.ResponseWriter, _ *http.Request) {
			fmt.Fprintln(w, "pong")
		})
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	token := signTestToken(priv, jwtlib.MapClaims{
		"sub":         "alice",
		"roles":       []string{"admin"},
		"permissions": []string{"posts:manage"},
	})

	for _, tc := range []struct {
		name string
		path string
		hdr  http.Header
		want int
	}{
		{"public", "/public", nil, http.StatusOK},
		{"jwt me", "/me", http.Header{"Authorization": []string{"Bearer " + token}}, http.StatusOK},
		{"jwt gate", "/posts/manage", http.Header{"Authorization": []string{"Bearer " + token}}, http.StatusOK},
		{"jwt missing", "/me", nil, http.StatusUnauthorized},
		{"apikey", "/internal/ping", http.Header{"X-API-Key": []string{"service-key-123"}}, http.StatusOK},
		{"apikey bad", "/internal/ping", http.Header{"X-API-Key": []string{"wrong"}}, http.StatusUnauthorized},
	} {
		req, _ := http.NewRequest(http.MethodGet, srv.URL+tc.path, nil)
		req.Header = tc.hdr
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Fatalf("%s: %v", tc.name, err)
		}
		resp.Body.Close()
		fmt.Printf("%-12s → %d\n", tc.name, resp.StatusCode)
		if resp.StatusCode != tc.want {
			log.Fatalf("%s: got %d want %d", tc.name, resp.StatusCode, tc.want)
		}
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

func newTestJWKS() (*rsa.PrivateKey, string) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(jwksDocument{Keys: []jwkKey{rsaPublicJWK("kid-1", &priv.PublicKey)}})
	}))
	return priv, srv.URL
}

func rsaPublicJWK(kid string, pub *rsa.PublicKey) jwkKey {
	n := base64.RawURLEncoding.EncodeToString(pub.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes())
	return jwkKey{Kty: "RSA", Kid: kid, N: n, E: e}
}

func signTestToken(priv *rsa.PrivateKey, claims jwtlib.MapClaims) string {
	if _, ok := claims["exp"]; !ok {
		claims["exp"] = time.Now().Add(time.Hour).Unix()
	}
	token := jwtlib.NewWithClaims(jwtlib.SigningMethodRS256, claims)
	token.Header["kid"] = "kid-1"
	s, err := token.SignedString(priv)
	if err != nil {
		log.Fatal(err)
	}
	return s
}
