package introspect

import (
	"context"
	"errors"
	"testing"

	adriver "github.com/godx-jp/godx-platform-framework/auth/driver"
)

func TestAuthenticateReturnsNotImplemented(t *testing.T) {
	g, err := adriver.New(context.Background(), adriver.Spec{
		Name:          adriver.DriverIntrospect,
		IntrospectURL: "https://idp.example.com/introspect",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = g.Authenticate(context.Background(), &adriver.CredentialRequest{Token: "opaque"})
	if !errors.Is(err, adriver.ErrNotImplemented) {
		t.Fatalf("err=%v", err)
	}
}

func TestShutdownBlocksAuthenticate(t *testing.T) {
	g, err := adriver.New(context.Background(), adriver.Spec{Name: adriver.DriverIntrospect})
	if err != nil {
		t.Fatal(err)
	}
	if err := g.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	_, err = g.Authenticate(context.Background(), &adriver.CredentialRequest{Token: "x"})
	if !errors.Is(err, adriver.ErrClosed) {
		t.Fatalf("err=%v", err)
	}
}

func TestDriverAutoRegistered(t *testing.T) {
	if adriver.Lookup(adriver.DriverIntrospect) == nil {
		t.Fatal("introspect driver not registered")
	}
}
