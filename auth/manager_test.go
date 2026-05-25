package auth

import (
	"context"
	"strings"
	"sync"
	"testing"

	adriver "github.com/godx-jp/godx-platform-framework/auth/driver"
)

type fakeGuard struct {
	name string
}

func (f *fakeGuard) Name() string { return f.name }

func (f *fakeGuard) Authenticate(_ context.Context, req *adriver.CredentialRequest) (*adriver.Principal, error) {
	if req == nil || req.Token == "" {
		return nil, adriver.ErrInvalidCredential
	}
	return &adriver.Principal{SubjectID: "sub-1", ActorKind: adriver.ActorHuman}, nil
}

func (f *fakeGuard) Shutdown(context.Context) error { return nil }

func TestManagerDefaultAndNamed(t *testing.T) {
	mgr := NewManager()
	jwt := &fakeGuard{name: "jwt"}
	api := &fakeGuard{name: "apikey"}
	if err := mgr.AddGuard("jwt", jwt); err != nil {
		t.Fatal(err)
	}
	if err := mgr.AddGuard("api", api); err != nil {
		t.Fatal(err)
	}
	if err := mgr.SetDefault("api"); err != nil {
		t.Fatal(err)
	}
	if mgr.Default().Name() != "apikey" {
		t.Fatalf("Default wrong: %v", mgr.Default())
	}
	got, err := mgr.Guard("jwt")
	if err != nil || got.Name() != "jwt" {
		t.Fatalf("jwt lookup wrong: %v err=%v", got, err)
	}
	if _, err := mgr.Guard("missing"); err == nil {
		t.Fatalf("missing should error")
	}
	names := mgr.Names()
	if len(names) != 2 || names[0] != "api" || names[1] != "jwt" {
		t.Fatalf("Names sort wrong: %v", names)
	}
}

func TestManagerAuthenticateDefaultAndNamed(t *testing.T) {
	mgr := NewManager()
	_ = mgr.AddGuard("jwt", &fakeGuard{name: "jwt"})
	_ = mgr.SetDefault("jwt")

	p, err := mgr.Authenticate(context.Background(), &adriver.CredentialRequest{Token: "t"})
	if err != nil || p.SubjectID != "sub-1" {
		t.Fatalf("default auth: p=%v err=%v", p, err)
	}
	p, err = mgr.Authenticate(context.Background(), &adriver.CredentialRequest{Guard: "jwt", Token: "t"})
	if err != nil || p.Guard != "jwt" {
		t.Fatalf("named auth: p=%v err=%v", p, err)
	}
	if _, err := mgr.Authenticate(context.Background(), nil); err == nil {
		t.Fatalf("nil request should error")
	}
}

func TestManagerDuplicateRejected(t *testing.T) {
	mgr := NewManager()
	g := &fakeGuard{name: "jwt"}
	if err := mgr.AddGuard("jwt", g); err != nil {
		t.Fatal(err)
	}
	if err := mgr.AddGuard("jwt", g); err == nil {
		t.Fatalf("duplicate should error")
	}
}

func TestManagerNilGuard(t *testing.T) {
	mgr := NewManager()
	if err := mgr.AddGuard("x", nil); err == nil {
		t.Fatalf("nil should error")
	}
	if err := mgr.AddGuard("", nil); err == nil {
		t.Fatalf("empty name should error")
	}
}

func TestManagerSetDefaultUnknown(t *testing.T) {
	mgr := NewManager()
	if err := mgr.SetDefault("nope"); err == nil {
		t.Fatalf("SetDefault unknown should error")
	}
}

func TestManagerShutdownNoop(t *testing.T) {
	mgr := NewManager()
	_ = mgr.AddGuard("jwt", &fakeGuard{name: "jwt"})
	if err := mgr.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
}

func TestManagerConcurrentAuthenticate(t *testing.T) {
	mgr := NewManager()
	_ = mgr.AddGuard("jwt", &fakeGuard{name: "jwt"})
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = mgr.Authenticate(context.Background(), &adriver.CredentialRequest{Token: "x"})
		}()
	}
	wg.Wait()
}

func TestGateDefineCheck(t *testing.T) {
	if err := Define("test-gate-auth", func(p *Principal) bool {
		return p != nil && p.SubjectID == "gate-user"
	}); err != nil {
		t.Fatal(err)
	}
	if !Check("test-gate-auth", &Principal{SubjectID: "gate-user"}) {
		t.Fatal("Check should pass for gate-user")
	}
	if Check("test-gate-auth", &Principal{SubjectID: "other"}) {
		t.Fatal("Check should fail for other user")
	}
	if Check("missing-gate", &Principal{SubjectID: "x"}) {
		t.Fatal("missing gate should return false")
	}
	if err := Define("test-gate-auth", func(p *Principal) bool { return true }); err == nil || !strings.Contains(err.Error(), "already defined") {
		t.Fatalf("duplicate Define err=%v", err)
	}
}
