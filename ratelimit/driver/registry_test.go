package driver

import (
	"context"
	"strings"
	"testing"
)

type fakeLimiter struct{ name string }

func (f *fakeLimiter) Name() string                             { return f.name }
func (*fakeLimiter) Allow(context.Context, string) (bool, error) { return true, nil }
func (*fakeLimiter) Reset(context.Context, string)               {}
func (*fakeLimiter) Shutdown(context.Context) error              { return nil }

func TestRegisterLookupNamesNew(t *testing.T) {
	Register("fake_ratelimit_test", func(ctx context.Context, spec Spec) (Limiter, error) {
		return &fakeLimiter{name: spec.Name}, nil
	})
	defer func() {
		regMu.Lock()
		delete(reg, "fake_ratelimit_test")
		regMu.Unlock()
	}()

	if Lookup("missing_xxx") != nil {
		t.Fatalf("Lookup missing")
	}
	if Lookup("fake_ratelimit_test") == nil {
		t.Fatalf("Lookup fake nil")
	}

	found := false
	for _, n := range Names() {
		if n == "fake_ratelimit_test" {
			found = true
		}
	}
	if !found {
		t.Fatalf("Names missing fake driver")
	}

	l, err := New(context.Background(), Spec{Name: "fake_ratelimit_test"})
	if err != nil || l == nil {
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
	if !strings.Contains(err.Error(), "ratelimit/drivers/neverregistered") {
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
	for _, n := range []string{DriverMemory, DriverRedis} {
		if n == "" {
			t.Fatalf("empty constant")
		}
	}
}
