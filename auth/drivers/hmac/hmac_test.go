package hmac_test

import (
	"context"
	"testing"
	"time"

	adriver "github.com/godx-jp/godx-platform-framework/auth/driver"
	_ "github.com/godx-jp/godx-platform-framework/auth/drivers/hmac"
	"github.com/godx-jp/godx-platform-framework/auth/token"
	jwtlib "github.com/golang-jwt/jwt/v5"
)

const testSecret = "01234567890123456789012345678901"

// signHS256 mints a token with arbitrary claims so tests can exercise edge
// cases (missing exp, past exp, future nbf, over-long TTL) that the higher-level
// token.IssueHS256 helper deliberately prevents.
func signHS256(t *testing.T, claims jwtlib.MapClaims) string {
	t.Helper()
	tok := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, claims)
	s, err := tok.SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return s
}

func newGuard(t *testing.T, spec adriver.Spec) adriver.Guard {
	t.Helper()
	spec.Name = adriver.DriverHMAC
	if spec.Secret == "" {
		spec.Secret = testSecret
	}
	g, err := adriver.New(context.Background(), spec)
	if err != nil {
		t.Fatalf("New guard: %v", err)
	}
	t.Cleanup(func() { _ = g.Shutdown(context.Background()) })
	return g
}

func baseClaims() jwtlib.MapClaims {
	now := time.Now()
	return jwtlib.MapClaims{
		"iss": "caller",
		"aud": "orders",
		"sub": "caller",
		"iat": now.Unix(),
		"exp": now.Add(time.Minute).Unix(),
	}
}

func TestAuthenticateValidToken(t *testing.T) {
	tok, err := token.IssueHS256(token.HMACOptions{
		Secret:   []byte(testSecret),
		Issuer:   "caller",
		Audience: "orders",
		Subject:  "caller",
		Claims:   map[string]any{"tenant_id": "t1"},
		TTL:      time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	g, err := adriver.New(context.Background(), adriver.Spec{
		Name:     adriver.DriverHMAC,
		Secret:   testSecret,
		Audience: "orders",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer g.Shutdown(context.Background())

	p, err := g.Authenticate(context.Background(), &adriver.CredentialRequest{Token: tok})
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if p.SubjectID != "caller" || p.ActorKind != adriver.ActorService {
		t.Fatalf("principal=%+v", p)
	}
	if p.Claims["tenant_id"] != "t1" {
		t.Fatalf("claims=%v", p.Claims)
	}
}

func TestAuthenticateRejectsWrongAudience(t *testing.T) {
	tok, _ := token.IssueHS256(token.HMACOptions{
		Secret: []byte(testSecret), Issuer: "a", Audience: "other", Subject: "a", TTL: time.Minute,
	})
	g, err := adriver.New(context.Background(), adriver.Spec{
		Name: adriver.DriverHMAC, Secret: testSecret, Audience: "orders",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer g.Shutdown(context.Background())
	_, err = g.Authenticate(context.Background(), &adriver.CredentialRequest{Token: tok})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRequiresMinSecretLength(t *testing.T) {
	_, err := adriver.New(context.Background(), adriver.Spec{
		Name: adriver.DriverHMAC, Secret: "short",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func authenticate(t *testing.T, g adriver.Guard, tok string) error {
	t.Helper()
	_, err := g.Authenticate(context.Background(), &adriver.CredentialRequest{Token: tok})
	return err
}

// TestRejectsTokenWithoutExp covers a token that omits exp entirely — it must
// be rejected now that exp is required.
func TestRejectsTokenWithoutExp(t *testing.T) {
	g := newGuard(t, adriver.Spec{Audience: "orders"})
	claims := baseClaims()
	delete(claims, "exp")
	if err := authenticate(t, g, signHS256(t, claims)); err == nil {
		t.Fatal("expected token without exp to be rejected")
	}
}

// TestRejectsTokenWithStringExp covers an exp claim encoded as a string, which
// the library cannot parse — it must be treated as invalid, not as absent.
func TestRejectsTokenWithStringExp(t *testing.T) {
	g := newGuard(t, adriver.Spec{Audience: "orders"})
	claims := baseClaims()
	claims["exp"] = "9999999999"
	if err := authenticate(t, g, signHS256(t, claims)); err == nil {
		t.Fatal("expected token with string exp to be rejected")
	}
}

// TestRejectsExpiredToken covers an exp in the past (beyond leeway).
func TestRejectsExpiredToken(t *testing.T) {
	g := newGuard(t, adriver.Spec{Audience: "orders", LeewaySeconds: 1})
	claims := baseClaims()
	claims["iat"] = time.Now().Add(-2 * time.Hour).Unix()
	claims["exp"] = time.Now().Add(-time.Hour).Unix()
	if err := authenticate(t, g, signHS256(t, claims)); err == nil {
		t.Fatal("expected expired token to be rejected")
	}
}

// TestRejectsFutureNbf covers an nbf claim in the future (beyond leeway).
func TestRejectsFutureNbf(t *testing.T) {
	g := newGuard(t, adriver.Spec{Audience: "orders", LeewaySeconds: 1})
	claims := baseClaims()
	claims["nbf"] = time.Now().Add(time.Hour).Unix()
	if err := authenticate(t, g, signHS256(t, claims)); err == nil {
		t.Fatal("expected token with future nbf to be rejected")
	}
}

// TestRejectsTokenExceedingMaxTTL covers exp-iat larger than the guard's
// configured MaxTokenTTL.
func TestRejectsTokenExceedingMaxTTL(t *testing.T) {
	g := newGuard(t, adriver.Spec{Audience: "orders", MaxTokenTTL: 5 * time.Minute})
	now := time.Now()
	claims := baseClaims()
	claims["iat"] = now.Unix()
	claims["exp"] = now.Add(time.Hour).Unix() // 60m > 5m cap
	if err := authenticate(t, g, signHS256(t, claims)); err == nil {
		t.Fatal("expected over-long token to be rejected")
	}
}

// TestAcceptsTokenWithinMaxTTL ensures a token whose lifetime fits the cap (and
// carries a valid past nbf) still authenticates.
func TestAcceptsTokenWithinMaxTTL(t *testing.T) {
	g := newGuard(t, adriver.Spec{Audience: "orders", MaxTokenTTL: 10 * time.Minute})
	now := time.Now()
	claims := baseClaims()
	claims["iat"] = now.Unix()
	claims["nbf"] = now.Add(-time.Minute).Unix()
	claims["exp"] = now.Add(5 * time.Minute).Unix() // 5m <= 10m cap
	if err := authenticate(t, g, signHS256(t, claims)); err != nil {
		t.Fatalf("expected valid token to authenticate, got %v", err)
	}
}
