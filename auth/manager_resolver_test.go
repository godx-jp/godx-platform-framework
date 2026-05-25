package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	adriver "github.com/godx-jp/godx-platform-framework/auth/driver"
)

func TestSetResolverRequiresRegisteredGuard(t *testing.T) {
	mgr := NewManager()
	if err := mgr.SetResolver("nope", BearerTokenResolver()); err == nil {
		t.Fatal("expected error for unknown guard")
	}
}

func TestSetResolverUsedByMiddlewarePath(t *testing.T) {
	ctx := context.Background()
	mgr := NewManager()
	g, err := adriver.New(ctx, adriver.Spec{
		Name: adriver.DriverAPIKey,
		Header: "X-Partner-Key",
		Keys: map[string]adriver.APIKeyEntry{
			"p": {SubjectID: "partner", Secret: "key"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = mgr.AddGuard("partner", g)
	_ = mgr.SetResolver("partner", APIKeyHeaderResolver("X-Partner-Key"))

	resolve, err := mgr.Resolver("partner")
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Partner-Key", "key")
	cred, err := resolve(r)
	if err != nil || cred.APIKey != "key" {
		t.Fatalf("cred=%+v err=%v", cred, err)
	}
}
