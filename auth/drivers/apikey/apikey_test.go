package apikey

import (
	"context"
	"errors"
	"testing"

	adriver "github.com/godx-jp/godx-platform-framework/auth/driver"
)

func TestAuthenticateValidKey(t *testing.T) {
	g, err := adriver.New(context.Background(), adriver.Spec{
		Name: adriver.DriverAPIKey,
		Header: "X-API-Key",
		Keys: map[string]adriver.APIKeyEntry{
			"svc-a": {
				SubjectID:   "svc-a",
				Secret:      "super-secret",
				ActorKind:   adriver.ActorService,
				Roles:       []string{"reader"},
				Permissions: []string{"orders:read"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	p, err := g.Authenticate(context.Background(), &adriver.CredentialRequest{APIKey: "super-secret"})
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if p.SubjectID != "svc-a" || p.ActorKind != adriver.ActorService {
		t.Fatalf("principal=%+v", p)
	}
	if len(p.Roles) != 1 || p.Roles[0] != "reader" {
		t.Fatalf("roles=%v", p.Roles)
	}
}

func TestAuthenticateInvalidKey(t *testing.T) {
	g, err := adriver.New(context.Background(), adriver.Spec{
		Name: adriver.DriverAPIKey,
		Keys: map[string]adriver.APIKeyEntry{
			"x": {SubjectID: "x", Secret: "good"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = g.Authenticate(context.Background(), &adriver.CredentialRequest{APIKey: "bad"})
	if !errors.Is(err, adriver.ErrInvalidCredential) {
		t.Fatalf("err=%v", err)
	}
}

func TestAuthenticateConstantTimeCompare(t *testing.T) {
	g, err := adriver.New(context.Background(), adriver.Spec{
		Name: adriver.DriverAPIKey,
		Keys: map[string]adriver.APIKeyEntry{
			"a": {SubjectID: "a", Secret: "aaaaaaaa"},
			"b": {SubjectID: "b", Secret: "bbbbbbbb"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := g.Authenticate(context.Background(), &adriver.CredentialRequest{APIKey: "bbbbbbbb"})
	if err != nil || p.SubjectID != "b" {
		t.Fatalf("second key: p=%+v err=%v", p, err)
	}
}

func TestDefaultHeader(t *testing.T) {
	g, err := adriver.New(context.Background(), adriver.Spec{
		Name: adriver.DriverAPIKey,
		Keys: map[string]adriver.APIKeyEntry{
			"k": {SubjectID: "k", Secret: "s"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if g.Name() != adriver.DriverAPIKey {
		t.Fatalf("name=%q", g.Name())
	}
}

func TestShutdown(t *testing.T) {
	g, err := adriver.New(context.Background(), adriver.Spec{
		Name: adriver.DriverAPIKey,
		Keys: map[string]adriver.APIKeyEntry{
			"k": {SubjectID: "k", Secret: "s"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := g.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := g.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	_, err = g.Authenticate(context.Background(), &adriver.CredentialRequest{APIKey: "s"})
	if !errors.Is(err, adriver.ErrClosed) {
		t.Fatalf("err=%v", err)
	}
}
