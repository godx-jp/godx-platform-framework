package hmac_test

import (
	"context"
	"testing"
	"time"

	adriver "github.com/godx-jp/godx-platform-framework/auth/driver"
	_ "github.com/godx-jp/godx-platform-framework/auth/drivers/hmac"
	"github.com/godx-jp/godx-platform-framework/auth/token"
)

const testSecret = "01234567890123456789012345678901"

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
