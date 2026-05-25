package auth

import (
	"context"
	"testing"

	adriver "github.com/godx-jp/godx-platform-framework/auth/driver"
)

func TestAuthenticateNoDefaultGuard(t *testing.T) {
	mgr := NewManager()
	_, err := mgr.Authenticate(context.Background(), &adriver.CredentialRequest{Token: "x"})
	if err == nil {
		t.Fatal("expected error without default guard")
	}
}

func TestAuthenticateSetsGuardOnPrincipal(t *testing.T) {
	mgr := NewManager()
	g := &fakeGuard{name: "jwt"}
	_ = mgr.AddGuard("jwt", g)
	_ = mgr.SetDefault("jwt")

	p, err := mgr.Authenticate(context.Background(), &adriver.CredentialRequest{Token: "t"})
	if err != nil || p.Guard != "jwt" {
		t.Fatalf("p=%+v err=%v", p, err)
	}
}
