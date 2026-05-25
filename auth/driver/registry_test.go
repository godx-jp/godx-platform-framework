package driver

import (
	"context"
	"strings"
	"testing"
)

type fakeGuard struct{ name string }

func (f *fakeGuard) Name() string { return f.name }

func (f *fakeGuard) Authenticate(context.Context, *CredentialRequest) (*Principal, error) {
	return &Principal{SubjectID: "x"}, nil
}

func (f *fakeGuard) Shutdown(context.Context) error { return nil }

func TestRegisterLookupNamesNew(t *testing.T) {
	Register("fake_auth_test", func(ctx context.Context, spec Spec) (Guard, error) {
		return &fakeGuard{name: spec.Name}, nil
	})
	defer func() {
		regMu.Lock()
		delete(reg, "fake_auth_test")
		regMu.Unlock()
	}()

	if Lookup("missing_xxx") != nil {
		t.Fatalf("Lookup missing")
	}
	if Lookup("fake_auth_test") == nil {
		t.Fatalf("Lookup fake nil")
	}

	found := false
	for _, n := range Names() {
		if n == "fake_auth_test" {
			found = true
		}
	}
	if !found {
		t.Fatalf("Names missing fake driver")
	}

	g, err := New(context.Background(), Spec{Name: "fake_auth_test"})
	if err != nil || g == nil {
		t.Fatalf("New: %v", err)
	}
}

func TestNewEmptyName(t *testing.T) {
	if _, err := New(context.Background(), Spec{}); err == nil {
		t.Fatalf("empty name should error")
	}
}

func TestNewUnknownNameMentionsImport(t *testing.T) {
	_, err := New(context.Background(), Spec{Name: "neverregistered"})
	if err == nil {
		t.Fatalf("unknown driver should error")
	}
	if !strings.Contains(err.Error(), "neverregistered") {
		t.Fatalf("error should mention name: %v", err)
	}
	if !strings.Contains(err.Error(), "auth/drivers/neverregistered") {
		t.Fatalf("error should hint at blank-import path: %v", err)
	}
}

func TestRegisterEmpty(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic")
		}
	}()
	Register("", nil)
}

func TestRegisterNil(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic")
		}
	}()
	Register("xx", nil)
}

func TestDriverNameConstants(t *testing.T) {
	for _, n := range []string{DriverJWT, DriverAPIKey, DriverIntrospect} {
		if n == "" {
			t.Fatalf("empty constant")
		}
	}
}
