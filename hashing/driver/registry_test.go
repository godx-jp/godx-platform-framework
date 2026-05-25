package driver

import (
	"context"
	"errors"
	"testing"
)

type fakeHasher struct{}

func (fakeHasher) Name() string                                       { return "fake" }
func (fakeHasher) Make(context.Context, string) (string, error)       { return "", nil }
func (fakeHasher) Check(context.Context, string, string) (bool, error) { return false, nil }
func (fakeHasher) NeedsRehash(string) bool                            { return false }
func (fakeHasher) Info(string) (Info, error)                          { return Info{}, nil }

func TestRegisterLookupNamesNew(t *testing.T) {
	Register("fake_hash_test", func(ctx context.Context, spec Spec) (Hasher, error) { return fakeHasher{}, nil })
	defer func() {
		regMu.Lock()
		delete(reg, "fake_hash_test")
		regMu.Unlock()
	}()

	if Lookup("missing_xxx") != nil {
		t.Fatalf("Lookup missing")
	}
	if Lookup("fake_hash_test") == nil {
		t.Fatalf("Lookup fake nil")
	}
	found := false
	for _, n := range Names() {
		if n == "fake_hash_test" {
			found = true
		}
	}
	if !found {
		t.Fatalf("Names missing fake driver")
	}

	h, err := New(context.Background(), Spec{Name: "fake_hash_test"})
	if err != nil || h == nil {
		t.Fatalf("New: %v", err)
	}
}

func TestNewEmptyName(t *testing.T) {
	if _, err := New(context.Background(), Spec{}); err == nil {
		t.Fatalf("empty name should error")
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
	es := []error{ErrInvalidHash, ErrUnknownFormat, ErrPasswordTooLong, ErrIncompatibleParams}
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
