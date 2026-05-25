package auth

import (
	"errors"
	"net/http"
	"testing"

	adriver "github.com/godx-jp/godx-platform-framework/auth/driver"
)

func TestBearerResolverEdges(t *testing.T) {
	resolve := BearerTokenResolver()

	if _, err := resolve(nil); !errors.Is(err, adriver.ErrInvalidCredential) {
		t.Fatalf("nil request: %v", err)
	}

	r, _ := http.NewRequest(http.MethodGet, "/", nil)
	if _, err := resolve(r); !errors.Is(err, adriver.ErrInvalidCredential) {
		t.Fatalf("missing header: %v", err)
	}

	r.Header.Set("Authorization", "Bearer ")
	if _, err := resolve(r); !errors.Is(err, adriver.ErrInvalidCredential) {
		t.Fatalf("empty token: %v", err)
	}

	r.Header.Set("Authorization", "Basic abc")
	if _, err := resolve(r); !errors.Is(err, adriver.ErrInvalidCredential) {
		t.Fatalf("basic auth: %v", err)
	}
}

func TestAPIKeyResolverEdges(t *testing.T) {
	resolve := APIKeyHeaderResolver("")

	r, _ := http.NewRequest(http.MethodGet, "/", nil)
	if _, err := resolve(nil); !errors.Is(err, adriver.ErrInvalidCredential) {
		t.Fatalf("nil request: %v", err)
	}
	if _, err := resolve(r); !errors.Is(err, adriver.ErrInvalidCredential) {
		t.Fatalf("missing key: %v", err)
	}

	r.Header.Set("X-API-Key", "  key  ")
	cred, err := resolve(r)
	if err != nil || cred.APIKey != "key" {
		t.Fatalf("trimmed key: cred=%+v err=%v", cred, err)
	}
}

func TestChainResolverEdges(t *testing.T) {
	fail := func(*http.Request) (*adriver.CredentialRequest, error) {
		return nil, adriver.ErrInvalidCredential
	}
	ok := func(*http.Request) (*adriver.CredentialRequest, error) {
		return &adriver.CredentialRequest{APIKey: "ok"}, nil
	}
	r, _ := http.NewRequest(http.MethodGet, "/", nil)

	chain := ChainResolver(fail, nil, ok)
	cred, err := chain(r)
	if err != nil || cred.APIKey != "ok" {
		t.Fatalf("second resolver: cred=%+v err=%v", cred, err)
	}

	allFail := ChainResolver(fail, fail)
	if _, err := allFail(r); !errors.Is(err, adriver.ErrInvalidCredential) {
		t.Fatalf("all fail: %v", err)
	}
}

func TestGuardResolverSetsName(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-API-Key", "k")
	resolve := GuardResolver("api", APIKeyHeaderResolver("X-API-Key"))
	cred, err := resolve(r)
	if err != nil || cred.Guard != "api" {
		t.Fatalf("cred=%+v err=%v", cred, err)
	}
}

func TestResolverForGuardUnknown(t *testing.T) {
	if _, err := ResolverForGuard("unknown", ""); err == nil {
		t.Fatal("expected error for unknown driver")
	}
}
