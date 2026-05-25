package token_test

import (
	"testing"
	"time"

	"github.com/godx-jp/godx-platform-framework/auth/token"
	jwtlib "github.com/golang-jwt/jwt/v5"
)

const testSecret = "01234567890123456789012345678901"

func TestIssueHS256(t *testing.T) {
	s, err := token.IssueHS256(token.HMACOptions{
		Secret:   []byte(testSecret),
		Issuer:   "issuer",
		Audience: "aud",
		Subject:  "sub",
		Claims:   map[string]any{"role": "admin"},
		TTL:      time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := jwtlib.Parse(s, func(t *jwtlib.Token) (any, error) {
		return []byte(testSecret), nil
	}, jwtlib.WithValidMethods([]string{"HS256"}))
	if err != nil {
		t.Fatal(err)
	}
	mc := parsed.Claims.(jwtlib.MapClaims)
	if mc["iss"] != "issuer" || mc["aud"] != "aud" || mc["sub"] != "sub" {
		t.Fatalf("claims=%v", mc)
	}
}

func TestIssueHS256RequiresFields(t *testing.T) {
	_, err := token.IssueHS256(token.HMACOptions{Secret: []byte(testSecret)})
	if err == nil {
		t.Fatal("expected error")
	}
}
