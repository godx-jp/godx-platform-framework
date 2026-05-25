package auth

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"

	adriver "github.com/godx-jp/godx-platform-framework/auth/driver"
)

func TestPrincipalContextRoundTrip(t *testing.T) {
	p := &Principal{
		SubjectID:   "user-42",
		ActorKind:   ActorHuman,
		Roles:       []string{"admin"},
		Permissions: []string{"read"},
		Guard:       "jwt",
	}
	ctx := ContextWithPrincipal(context.Background(), p)
	got, ok := PrincipalFromContext(ctx)
	if !ok || got.SubjectID != "user-42" || got.Guard != "jwt" {
		t.Fatalf("PrincipalFromContext: %+v ok=%v", got, ok)
	}
	sub, ok := SubjectIDFromContext(ctx)
	if !ok || sub != "user-42" {
		t.Fatalf("SubjectIDFromContext=%q ok=%v", sub, ok)
	}
}

func TestBearerResolver(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	r.Header.Set("Authorization", "Bearer tok123")
	cred, err := BearerTokenResolver()(r)
	if err != nil || cred.Token != "tok123" {
		t.Fatalf("Bearer: cred=%+v err=%v", cred, err)
	}
	r.Header.Set("Authorization", "Basic x")
	if _, err := BearerTokenResolver()(r); !errors.Is(err, adriver.ErrInvalidCredential) {
		t.Fatalf("Basic should fail: %v", err)
	}
}

func TestAPIKeyResolver(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	r.Header.Set("X-API-Key", "abc")
	cred, err := APIKeyHeaderResolver("X-API-Key")(r)
	if err != nil || cred.APIKey != "abc" {
		t.Fatalf("APIKey: cred=%+v err=%v", cred, err)
	}
}

func TestManagerGateIntegration(t *testing.T) {
	ctx := context.Background()
	g, err := adriver.New(ctx, adriver.Spec{
		Name: adriver.DriverAPIKey,
		Keys: map[string]adriver.APIKeyEntry{
			"bot": {SubjectID: "bot", Secret: "s3cr3t"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	mgr := NewManager()
	_ = mgr.AddGuard("api", g)
	_ = mgr.SetDefault("api")

	r, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	r.Header.Set("X-API-Key", "s3cr3t")
	fn := ManagerGate(mgr, "api", APIKeyHeaderResolver("X-API-Key"))
	p, err := fn(r)
	if err != nil || p.SubjectID != "bot" {
		t.Fatalf("ManagerGate: p=%+v err=%v", p, err)
	}
}

func TestAPIKeyGuardConcurrent(t *testing.T) {
	ctx := context.Background()
	g, err := adriver.New(ctx, adriver.Spec{
		Name: adriver.DriverAPIKey,
		Keys: map[string]adriver.APIKeyEntry{
			"svc": {SubjectID: "svc", Secret: "key"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = g.Authenticate(ctx, &adriver.CredentialRequest{APIKey: "key"})
			_, _ = g.Authenticate(ctx, &adriver.CredentialRequest{APIKey: "wrong"})
		}()
	}
	wg.Wait()
}

func TestLightDriversAutoRegister(t *testing.T) {
	if adriver.Lookup(adriver.DriverJWT) == nil {
		t.Fatalf("jwt driver not auto-registered")
	}
	if adriver.Lookup(adriver.DriverAPIKey) == nil {
		t.Fatalf("apikey driver not auto-registered")
	}
	if adriver.Lookup(adriver.DriverHMAC) == nil {
		t.Fatalf("hmac driver not auto-registered")
	}
}

func TestAPIKeyGuardShutdownBlocksOps(t *testing.T) {
	ctx := context.Background()
	g, err := adriver.New(ctx, adriver.Spec{
		Name: adriver.DriverAPIKey,
		Keys: map[string]adriver.APIKeyEntry{
			"x": {SubjectID: "x", Secret: "k"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := g.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	_, err = g.Authenticate(ctx, &adriver.CredentialRequest{APIKey: "k"})
	if !errors.Is(err, adriver.ErrClosed) {
		t.Fatalf("Authenticate after Shutdown err=%v", err)
	}
}
