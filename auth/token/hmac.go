// Package token provides helpers to mint credentials for tests and dev tooling.
// Production services should verify tokens via auth guards, not issue them at runtime.
package token

import (
	"errors"
	"fmt"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
)

const (
	// MinSecretLen is the minimum byte length for HS256 shared secrets.
	MinSecretLen = 32
	// DefaultMaxTTL is applied when Options.MaxTTL is zero.
	DefaultMaxTTL = 5 * time.Minute
)

// HMACOptions configures HS256 token issuance.
type HMACOptions struct {
	Secret   []byte
	Issuer   string
	Audience string
	Subject  string
	// Claims are merged into the JWT payload after registered claims.
	Claims map[string]any
	TTL    time.Duration
	MaxTTL time.Duration
}

// IssueHS256 signs a JWT with HS256 using the given options.
func IssueHS256(opts HMACOptions) (string, error) {
	if len(opts.Secret) < MinSecretLen {
		return "", fmt.Errorf("token: secret must be at least %d bytes", MinSecretLen)
	}
	if opts.Issuer == "" || opts.Audience == "" || opts.Subject == "" {
		return "", errors.New("token: issuer, audience, and subject are required")
	}
	maxTTL := opts.MaxTTL
	if maxTTL <= 0 {
		maxTTL = DefaultMaxTTL
	}
	ttl := opts.TTL
	if ttl <= 0 || ttl > maxTTL {
		ttl = maxTTL
	}
	now := time.Now()
	mc := jwtlib.MapClaims{
		"iss": opts.Issuer,
		"aud": opts.Audience,
		"sub": opts.Subject,
		"iat": now.Unix(),
		"exp": now.Add(ttl).Unix(),
	}
	for k, v := range opts.Claims {
		mc[k] = v
	}
	tok := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, mc)
	return tok.SignedString(opts.Secret)
}
