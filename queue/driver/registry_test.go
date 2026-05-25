package driver

import (
	"context"
	"testing"
)

func TestRegisterLookup(t *testing.T) {
	const testName = "test-driver-registry"
	Register(testName, func(ctx context.Context, spec Spec) (Backend, error) {
		return nil, nil
	})
	if Lookup(testName) == nil {
		t.Fatal("Lookup returned nil")
	}
	names := Names()
	found := false
	for _, n := range names {
		if n == testName {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Names() missing %q: %v", testName, names)
	}
}

func TestNewUnknownDriver(t *testing.T) {
	_, err := New(context.Background(), Spec{Name: "does-not-exist-xyz"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewEmptyName(t *testing.T) {
	_, err := New(context.Background(), Spec{})
	if err == nil {
		t.Fatal("expected error")
	}
}
