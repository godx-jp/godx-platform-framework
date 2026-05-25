package apikey

import (
	"context"
	"errors"
	"testing"

	adriver "github.com/godx-jp/godx-platform-framework/auth/driver"
)

func TestAuthenticateEmptyKeysMap(t *testing.T) {
	g, err := adriver.New(context.Background(), adriver.Spec{
		Name: adriver.DriverAPIKey,
		Keys: map[string]adriver.APIKeyEntry{},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = g.Authenticate(context.Background(), &adriver.CredentialRequest{APIKey: "anything"})
	if !errors.Is(err, adriver.ErrInvalidCredential) {
		t.Fatalf("err=%v", err)
	}
}

func TestAuthenticateEmptyAPIKey(t *testing.T) {
	g, err := adriver.New(context.Background(), adriver.Spec{
		Name: adriver.DriverAPIKey,
		Keys: map[string]adriver.APIKeyEntry{
			"s": {SubjectID: "s", Secret: "k"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = g.Authenticate(context.Background(), &adriver.CredentialRequest{APIKey: ""})
	if !errors.Is(err, adriver.ErrInvalidCredential) {
		t.Fatalf("err=%v", err)
	}
}

func TestAuthenticateWithRolesAndPermissions(t *testing.T) {
	g, err := adriver.New(context.Background(), adriver.Spec{
		Name: adriver.DriverAPIKey,
		Keys: map[string]adriver.APIKeyEntry{
			"bot": {
				SubjectID:   "bot-1",
				Secret:      "sekret",
				ActorKind:   adriver.ActorDevice,
				Roles:       []string{"automation"},
				Permissions: []string{"run:job"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := g.Authenticate(context.Background(), &adriver.CredentialRequest{APIKey: "sekret"})
	if err != nil {
		t.Fatal(err)
	}
	if p.SubjectID != "bot-1" || p.ActorKind != adriver.ActorDevice {
		t.Fatalf("p=%+v", p)
	}
	if len(p.Roles) != 1 || len(p.Permissions) != 1 {
		t.Fatalf("roles=%v perms=%v", p.Roles, p.Permissions)
	}
}

func TestSkipsEntriesWithEmptySecret(t *testing.T) {
	g, err := adriver.New(context.Background(), adriver.Spec{
		Name: adriver.DriverAPIKey,
		Keys: map[string]adriver.APIKeyEntry{
			"bad": {SubjectID: "bad", Secret: ""},
			"ok":  {SubjectID: "ok", Secret: "key"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := g.Authenticate(context.Background(), &adriver.CredentialRequest{APIKey: "key"})
	if err != nil || p.SubjectID != "ok" {
		t.Fatalf("p=%+v err=%v", p, err)
	}
}
