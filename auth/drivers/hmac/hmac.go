// Package hmac validates Bearer JWTs signed with a shared HS256 secret.
package hmac

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	adriver "github.com/godx-jp/godx-platform-framework/auth/driver"
	jwtlib "github.com/golang-jwt/jwt/v5"
)

const (
	defaultLeewaySeconds = 30
	minSecretLen         = 32
)

func init() {
	adriver.Register(adriver.DriverHMAC, func(_ context.Context, spec adriver.Spec) (adriver.Guard, error) {
		secret := []byte(strings.TrimSpace(spec.Secret))
		if len(secret) < minSecretLen {
			return nil, fmt.Errorf("hmac: secret must be at least %d bytes", minSecretLen)
		}
		leeway := spec.LeewaySeconds
		if leeway <= 0 {
			leeway = defaultLeewaySeconds
		}
		subjectClaim := strings.TrimSpace(spec.SubjectClaim)
		if subjectClaim == "" {
			subjectClaim = "sub"
		}
		rolesClaim := strings.TrimSpace(spec.RolesClaim)
		if rolesClaim == "" {
			rolesClaim = "roles"
		}
		permsClaim := strings.TrimSpace(spec.PermissionsClaim)
		if permsClaim == "" {
			permsClaim = "permissions"
		}
		return &guard{
			secret:           secret,
			issuer:           strings.TrimSpace(spec.Issuer),
			audience:         strings.TrimSpace(spec.Audience),
			leewaySeconds:    leeway,
			maxTokenTTL:      spec.MaxTokenTTL,
			subjectClaim:     subjectClaim,
			rolesClaim:       rolesClaim,
			permissionsClaim: permsClaim,
			actorKindClaim:   strings.TrimSpace(spec.ActorKindClaim),
		}, nil
	})
}

type guard struct {
	secret           []byte
	issuer           string
	audience         string
	leewaySeconds    int64
	maxTokenTTL      time.Duration
	subjectClaim     string
	rolesClaim       string
	permissionsClaim string
	actorKindClaim   string

	mu     sync.Mutex
	closed bool
}

func (g *guard) Name() string { return adriver.DriverHMAC }

func (g *guard) Authenticate(_ context.Context, req *adriver.CredentialRequest) (*adriver.Principal, error) {
	if err := g.checkOpen(); err != nil {
		return nil, err
	}
	if req == nil || strings.TrimSpace(req.Token) == "" {
		return nil, adriver.ErrInvalidCredential
	}
	mc, err := g.parseToken(strings.TrimSpace(req.Token))
	if err != nil {
		return nil, adriver.ErrInvalidCredential
	}
	if err := g.validateTimeClaims(mc, time.Now()); err != nil {
		return nil, adriver.ErrInvalidCredential
	}
	if g.issuer != "" {
		iss, _ := mc["iss"].(string)
		if iss != g.issuer {
			return nil, adriver.ErrInvalidCredential
		}
	}
	if g.audience != "" && !audienceContains(mc, g.audience) {
		return nil, adriver.ErrInvalidCredential
	}
	subject, _ := stringClaim(mc, g.subjectClaim)
	if subject == "" {
		return nil, adriver.ErrInvalidCredential
	}
	actor := adriver.ActorService
	if g.actorKindClaim != "" {
		if v, ok := stringClaim(mc, g.actorKindClaim); ok && v != "" {
			actor = adriver.ActorKind(v)
		}
	}
	rawClaims := map[string]any{}
	for k, v := range mc {
		rawClaims[k] = v
	}
	return &adriver.Principal{
		SubjectID:   subject,
		ActorKind:   actor,
		Roles:       stringSliceClaim(mc, g.rolesClaim),
		Permissions: stringSliceClaim(mc, g.permissionsClaim),
		Claims:      rawClaims,
	}, nil
}

func (g *guard) parseToken(tokenStr string) (jwtlib.MapClaims, error) {
	parsed, err := jwtlib.Parse(tokenStr, func(t *jwtlib.Token) (any, error) {
		if _, ok := t.Method.(*jwtlib.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return g.secret, nil
	}, jwtlib.WithValidMethods([]string{"HS256"}), jwtlib.WithExpirationRequired())
	if err != nil {
		return nil, err
	}
	mc, ok := parsed.Claims.(jwtlib.MapClaims)
	if !ok || !parsed.Valid {
		return nil, adriver.ErrInvalidCredential
	}
	return mc, nil
}

func (g *guard) validateTimeClaims(mc jwtlib.MapClaims, now time.Time) error {
	leeway := g.leewaySeconds

	var iat int64
	iatPresent := false
	if v, status := timeClaim(mc, "iat"); status == claimInvalid {
		return adriver.ErrInvalidCredential
	} else if status == claimPresent {
		iat = v
		iatPresent = true
		if iat > now.Unix()+leeway {
			return adriver.ErrInvalidCredential
		}
	}

	// nbf: if present, the token is not yet valid until nbf (minus leeway).
	if v, status := timeClaim(mc, "nbf"); status == claimInvalid {
		return adriver.ErrInvalidCredential
	} else if status == claimPresent {
		if v > now.Unix()+leeway {
			return adriver.ErrInvalidCredential
		}
	}

	// exp is mandatory: absent or unparseable both reject. (jwtlib's
	// WithExpirationRequired already rejects absent exp, but enforce it here
	// too so a string-encoded exp is treated as invalid, not as "absent".)
	exp, status := timeClaim(mc, "exp")
	if status != claimPresent {
		return adriver.ErrInvalidCredential
	}
	if exp < now.Unix()-leeway {
		return adriver.ErrInvalidCredential
	}

	// Enforce a maximum lifetime: reject tokens minted with an over-long TTL.
	if g.maxTokenTTL > 0 && iatPresent {
		if exp-iat > int64(g.maxTokenTTL.Seconds()) {
			return adriver.ErrInvalidCredential
		}
	}
	return nil
}

type claimStatus int

const (
	claimAbsent claimStatus = iota
	claimPresent
	claimInvalid
)

// timeClaim distinguishes an absent numeric time claim from one that is present
// but cannot be parsed (e.g. string-encoded), which must be treated as invalid
// rather than silently skipped.
func timeClaim(claims jwtlib.MapClaims, key string) (int64, claimStatus) {
	v, ok := claims[key]
	if !ok {
		return 0, claimAbsent
	}
	switch t := v.(type) {
	case float64:
		return int64(t), claimPresent
	case int64:
		return t, claimPresent
	case int:
		return int64(t), claimPresent
	default:
		return 0, claimInvalid
	}
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

func stringClaim(claims jwtlib.MapClaims, key string) (string, bool) {
	v, ok := claims[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok && s != ""
}

func stringSliceClaim(claims jwtlib.MapClaims, key string) []string {
	v, ok := claims[key]
	if !ok {
		return nil
	}
	switch t := v.(type) {
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return append([]string(nil), t...)
	case string:
		if t != "" {
			return []string{t}
		}
	}
	return nil
}

func audienceContains(claims jwtlib.MapClaims, want string) bool {
	v, ok := claims["aud"]
	if !ok {
		return false
	}
	switch t := v.(type) {
	case string:
		return t == want
	case []any:
		for _, item := range t {
			if s, ok := item.(string); ok && s == want {
				return true
			}
		}
	case []string:
		for _, s := range t {
			if s == want {
				return true
			}
		}
	}
	return false
}
