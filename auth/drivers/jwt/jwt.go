// Package jwt validates Bearer JWTs against a JWKS endpoint.
package jwt

import (
	"context"
	"crypto"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	adriver "github.com/godx-jp/godx-platform-framework/auth/driver"
	jwtlib "github.com/golang-jwt/jwt/v5"
)

const (
	defaultRolesClaim       = "roles"
	defaultPermissionsClaim = "permissions"
	defaultSubjectClaim     = "sub"
)

func init() {
	adriver.Register(adriver.DriverJWT, func(_ context.Context, spec adriver.Spec) (adriver.Guard, error) {
		if strings.TrimSpace(spec.JWKSURL) == "" {
			return nil, fmt.Errorf("jwt: JWKSURL is required")
		}
		rolesClaim := strings.TrimSpace(spec.RolesClaim)
		if rolesClaim == "" {
			rolesClaim = defaultRolesClaim
		}
		permsClaim := strings.TrimSpace(spec.PermissionsClaim)
		if permsClaim == "" {
			permsClaim = defaultPermissionsClaim
		}
		subjectClaim := strings.TrimSpace(spec.SubjectClaim)
		if subjectClaim == "" {
			subjectClaim = defaultSubjectClaim
		}
		client := &http.Client{Timeout: 10 * time.Second}
		return &guard{
			jwksURL:          spec.JWKSURL,
			issuer:           strings.TrimSpace(spec.Issuer),
			audience:         strings.TrimSpace(spec.Audience),
			rolesClaim:       rolesClaim,
			permissionsClaim: permsClaim,
			subjectClaim:     subjectClaim,
			actorKindClaim:   strings.TrimSpace(spec.ActorKindClaim),
			client:           client,
			keys:             map[string]crypto.PublicKey{},
		}, nil
	})
}

type guard struct {
	jwksURL          string
	issuer           string
	audience         string
	rolesClaim       string
	permissionsClaim string
	subjectClaim     string
	actorKindClaim   string
	client           *http.Client

	mu     sync.Mutex
	keys   map[string]crypto.PublicKey
	closed bool
}

func (g *guard) Name() string { return adriver.DriverJWT }

func (g *guard) Authenticate(ctx context.Context, req *adriver.CredentialRequest) (*adriver.Principal, error) {
	if err := g.checkOpen(); err != nil {
		return nil, err
	}
	if req == nil || strings.TrimSpace(req.Token) == "" {
		return nil, adriver.ErrInvalidCredential
	}
	tokenStr := strings.TrimSpace(req.Token)
	parser := jwtlib.NewParser(jwtlib.WithValidMethods([]string{"RS256", "RS384", "RS512", "ES256", "ES384", "ES512"}))
	token, err := parser.Parse(tokenStr, g.keyFunc(ctx))
	if err != nil {
		return nil, adriver.ErrInvalidCredential
	}
	if !token.Valid {
		return nil, adriver.ErrInvalidCredential
	}
	claims, ok := token.Claims.(jwtlib.MapClaims)
	if !ok {
		return nil, adriver.ErrInvalidCredential
	}
	if g.issuer != "" {
		iss, _ := claims.GetIssuer()
		if iss != g.issuer {
			return nil, adriver.ErrInvalidCredential
		}
	}
	if g.audience != "" {
		if !claimAudienceContains(claims, g.audience) {
			return nil, adriver.ErrInvalidCredential
		}
	}
	subject, _ := stringClaim(claims, g.subjectClaim)
	if subject == "" {
		return nil, adriver.ErrInvalidCredential
	}
	actor := adriver.ActorHuman
	if g.actorKindClaim != "" {
		if v, ok := stringClaim(claims, g.actorKindClaim); ok && v != "" {
			actor = adriver.ActorKind(v)
		}
	}
	roles := stringSliceClaim(claims, g.rolesClaim)
	perms := stringSliceClaim(claims, g.permissionsClaim)
	rawClaims := map[string]any{}
	for k, v := range claims {
		rawClaims[k] = v
	}
	return &adriver.Principal{
		SubjectID:   subject,
		ActorKind:   actor,
		Roles:       roles,
		Permissions: perms,
		Claims:      rawClaims,
	}, nil
}

func (g *guard) keyFunc(ctx context.Context) jwtlib.Keyfunc {
	return func(token *jwtlib.Token) (any, error) {
		kid, _ := token.Header["kid"].(string)
		key, err := g.lookupKey(ctx, kid)
		if err != nil {
			return nil, err
		}
		return key, nil
	}
}

func (g *guard) lookupKey(ctx context.Context, kid string) (crypto.PublicKey, error) {
	if key, ok := g.cachedKey(kid); ok {
		return key, nil
	}
	if err := g.refreshJWKS(ctx); err != nil {
		return nil, err
	}
	if key, ok := g.cachedKey(kid); ok {
		return key, nil
	}
	if err := g.refreshJWKS(ctx); err != nil {
		return nil, err
	}
	if key, ok := g.cachedKey(kid); ok {
		return key, nil
	}
	return nil, adriver.ErrInvalidCredential
}

func (g *guard) cachedKey(kid string) (crypto.PublicKey, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	key, ok := g.keys[kid]
	return key, ok
}

func (g *guard) refreshJWKS(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.jwksURL, nil)
	if err != nil {
		return err
	}
	resp, err := g.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("jwt: jwks fetch status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	var doc jwksDocument
	if err := json.Unmarshal(body, &doc); err != nil {
		return err
	}
	keys := map[string]crypto.PublicKey{}
	for _, k := range doc.Keys {
		pub, err := k.publicKey()
		if err != nil {
			continue
		}
		keys[k.Kid] = pub
	}
	g.mu.Lock()
	for kid, pub := range keys {
		g.keys[kid] = pub
	}
	g.mu.Unlock()
	return nil
}

func (g *guard) Shutdown(context.Context) error {
	g.mu.Lock()
	g.closed = true
	g.mu.Unlock()
	return nil
}

func (g *guard) checkOpen() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed {
		return adriver.ErrClosed
	}
	return nil
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

func (k jwkKey) publicKey() (crypto.PublicKey, error) {
	if strings.ToUpper(k.Kty) != "RSA" {
		return nil, errors.New("jwt: unsupported key type")
	}
	nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, err
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, err
	}
	n := new(big.Int).SetBytes(nBytes)
	e := new(big.Int).SetBytes(eBytes).Int64()
	if e <= 0 || e > int64(^uint(0)>>1) {
		return nil, errors.New("jwt: invalid exponent")
	}
	return &rsa.PublicKey{N: n, E: int(e)}, nil
}

func stringClaim(claims jwtlib.MapClaims, key string) (string, bool) {
	v, ok := claims[key]
	if !ok {
		return "", false
	}
	switch t := v.(type) {
	case string:
		return t, true
	default:
		return fmt.Sprint(t), true
	}
}

func stringSliceClaim(claims jwtlib.MapClaims, key string) []string {
	v, ok := claims[key]
	if !ok {
		return nil
	}
	switch t := v.(type) {
	case []string:
		return append([]string(nil), t...)
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			out = append(out, fmt.Sprint(item))
		}
		return out
	case string:
		if strings.TrimSpace(t) == "" {
			return nil
		}
		return []string{t}
	default:
		return nil
	}
}

func claimAudienceContains(claims jwtlib.MapClaims, want string) bool {
	aud, err := claims.GetAudience()
	if err == nil {
		for _, a := range aud {
			if a == want {
				return true
			}
		}
	}
	if raw, ok := claims["aud"]; ok {
		switch t := raw.(type) {
		case string:
			return t == want
		case []any:
			for _, item := range t {
				if fmt.Sprint(item) == want {
					return true
				}
			}
		}
	}
	return false
}
