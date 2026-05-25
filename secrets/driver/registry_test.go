package driver

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeStore struct{ name string }

func (f *fakeStore) Name() string                                  { return f.name }
func (*fakeStore) Get(context.Context, string) ([]byte, error)     { return nil, ErrNotFound }
func (*fakeStore) Put(context.Context, string, []byte) error       { return ErrReadOnly }
func (*fakeStore) Forget(context.Context, string) error            { return ErrReadOnly }
func (*fakeStore) List(context.Context) ([]string, error)          { return nil, ErrListNotSupported }
func (*fakeStore) Shutdown(context.Context) error                  { return nil }

func TestRegisterLookupNamesNew(t *testing.T) {
	Register("fake_secrets_test", func(ctx context.Context, spec Spec) (Store, error) {
		return &fakeStore{name: spec.Name}, nil
	})
	defer func() {
		regMu.Lock()
		delete(reg, "fake_secrets_test")
		regMu.Unlock()
	}()

	if Lookup("missing_xxx") != nil {
		t.Fatalf("Lookup missing")
	}
	if Lookup("fake_secrets_test") == nil {
		t.Fatalf("Lookup fake nil")
	}

	found := false
	for _, n := range Names() {
		if n == "fake_secrets_test" {
			found = true
		}
	}
	if !found {
		t.Fatalf("Names missing fake driver")
	}

	s, err := New(context.Background(), Spec{Name: "fake_secrets_test"})
	if err != nil || s == nil {
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
	if !strings.Contains(err.Error(), "secrets/drivers/neverregistered") {
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

func TestSentinelErrorsDistinct(t *testing.T) {
	es := []error{ErrNotFound, ErrReadOnly, ErrListNotSupported, ErrClosed}
	for i, a := range es {
		for j, b := range es {
			if i == j {
				continue
			}
			if errors.Is(a, b) {
				t.Fatalf("collide %v / %v", a, b)
			}
		}
	}
}

func TestDriverNameConstants(t *testing.T) {
	for _, n := range []string{DriverEnv, DriverFile, DriverVault, DriverGCPSM, DriverAWSSM} {
		if n == "" {
			t.Fatalf("empty constant")
		}
	}
}
