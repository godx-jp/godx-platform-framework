package driver

import (
	"context"
	"errors"
	"testing"
)

type fakeCipher struct{}

func (fakeCipher) Name() string                                                         { return "fake" }
func (fakeCipher) KeySize() int                                                          { return 32 }
func (fakeCipher) Encrypt(context.Context, []byte, []byte) ([]byte, error)              { return nil, nil }
func (fakeCipher) Decrypt(context.Context, []byte, []byte) ([]byte, error)              { return nil, nil }

func TestRegisterLookupNamesNew(t *testing.T) {
	Register("fake_enc_test", func(ctx context.Context, spec Spec) (Cipher, error) { return fakeCipher{}, nil })
	defer func() {
		regMu.Lock()
		delete(reg, "fake_enc_test")
		regMu.Unlock()
	}()

	if Lookup("missing_xxx") != nil {
		t.Fatalf("Lookup missing")
	}
	if Lookup("fake_enc_test") == nil {
		t.Fatalf("Lookup fake nil")
	}
	names := Names()
	found := false
	for _, n := range names {
		if n == "fake_enc_test" {
			found = true
		}
	}
	if !found {
		t.Fatalf("Names missing fake")
	}
	c, err := New(context.Background(), Spec{Name: "fake_enc_test"})
	if err != nil || c == nil {
		t.Fatalf("New: %v", err)
	}
}

func TestNewEmptyName(t *testing.T) {
	if _, err := New(context.Background(), Spec{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestNewMissingDriver(t *testing.T) {
	if _, err := New(context.Background(), Spec{Name: "definitely-missing"}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRegisterPanics(t *testing.T) {
	for _, tc := range []struct {
		name string
		fn   func()
	}{
		{"empty name", func() { Register("", func(context.Context, Spec) (Cipher, error) { return nil, nil }) }},
		{"nil ctor", func() { Register("xx", nil) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Fatalf("expected panic")
				}
			}()
			tc.fn()
		})
	}
}

func TestSentinelErrorsDistinct(t *testing.T) {
	es := []error{ErrAuthFailed, ErrInvalidKeySize, ErrShortCiphertext, ErrInvalidToken, ErrUnknownKey}
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
