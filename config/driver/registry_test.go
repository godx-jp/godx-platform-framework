package driver

import (
	"context"
	"errors"
	"testing"
)

type fakeSource struct{ closed bool }

func (f *fakeSource) Name() string                                     { return "fake" }
func (f *fakeSource) Load(context.Context) (map[string]any, error)     { return map[string]any{}, nil }
func (f *fakeSource) Shutdown(context.Context) error                   { f.closed = true; return nil }

func TestRegisterLookupNamesNew(t *testing.T) {
	Register("fake_test_driver", func(ctx context.Context, spec Spec) (Source, error) {
		return &fakeSource{}, nil
	})
	defer func() {
		regMu.Lock()
		delete(reg, "fake_test_driver")
		regMu.Unlock()
	}()

	if Lookup("missing_xxx") != nil {
		t.Fatalf("Lookup missing should be nil")
	}
	if Lookup("fake_test_driver") == nil {
		t.Fatalf("Lookup fake should not be nil")
	}
	names := Names()
	found := false
	for _, n := range names {
		if n == "fake_test_driver" {
			found = true
		}
	}
	if !found {
		t.Fatalf("Names did not include registered driver")
	}

	src, err := New(context.Background(), Spec{Name: "fake_test_driver"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if src.Name() != "fake" {
		t.Fatalf("constructed Source.Name unexpected: %q", src.Name())
	}
}

func TestNewEmptyName(t *testing.T) {
	if _, err := New(context.Background(), Spec{}); err == nil {
		t.Fatalf("New with empty name should error")
	}
}

func TestNewMissingDriver(t *testing.T) {
	_, err := New(context.Background(), Spec{Name: "definitely-not-registered-xxx"})
	if err == nil {
		t.Fatalf("New with missing driver should error")
	}
}

func TestRegisterEmpty(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("Register with empty name should panic")
		}
	}()
	Register("", func(ctx context.Context, spec Spec) (Source, error) { return nil, nil })
}

func TestRegisterNil(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("Register with nil constructor should panic")
		}
	}()
	Register("zz", nil)
}

func TestSentinelErrorsDistinct(t *testing.T) {
	es := []error{ErrNotSupported, ErrClosed, ErrFileMissing, ErrUnsupportedFormat}
	for i, a := range es {
		for j, b := range es {
			if i == j {
				continue
			}
			if errors.Is(a, b) {
				t.Fatalf("sentinel errors %v and %v collide", a, b)
			}
		}
	}
}
