package vault

import (
	"context"
	"testing"

	sdriver "github.com/godx-jp/godx-platform-framework/secrets/driver"
)

func TestRegisteredOnImport(t *testing.T) {
	if sdriver.Lookup(sdriver.DriverVault) == nil {
		t.Fatalf("vault driver not registered")
	}
}

func TestConstructorValidatesAddress(t *testing.T) {
	c := sdriver.Lookup(sdriver.DriverVault)
	if _, err := c(context.Background(), sdriver.Spec{Name: sdriver.DriverVault}); err == nil {
		t.Fatalf("expected error for missing address")
	}
}

func TestConstructorDefaults(t *testing.T) {
	c := sdriver.Lookup(sdriver.DriverVault)
	s, err := c(context.Background(), sdriver.Spec{
		Name:    sdriver.DriverVault,
		Address: "http://127.0.0.1:0",
		Token:   "irrelevant",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if s.Name() != sdriver.DriverVault {
		t.Fatalf("name=%q", s.Name())
	}
	_ = s.Shutdown(context.Background())
}

func TestShutdownIdempotent(t *testing.T) {
	s, err := New(context.Background(), "http://127.0.0.1:0", "tok", "secret", "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := s.Shutdown(context.Background()); err != nil {
		t.Fatalf("first: %v", err)
	}
	if err := s.Shutdown(context.Background()); err != nil {
		t.Fatalf("second: %v", err)
	}
}

func TestAfterShutdown(t *testing.T) {
	s, _ := New(context.Background(), "http://127.0.0.1:0", "tok", "secret", "")
	_ = s.Shutdown(context.Background())
	if _, err := s.Get(context.Background(), "k"); err != sdriver.ErrClosed {
		t.Fatalf("Get err=%v", err)
	}
	if err := s.Put(context.Background(), "k", []byte("v")); err != sdriver.ErrClosed {
		t.Fatalf("Put err=%v", err)
	}
	if err := s.Forget(context.Background(), "k"); err != sdriver.ErrClosed {
		t.Fatalf("Forget err=%v", err)
	}
	if _, err := s.List(context.Background()); err != sdriver.ErrClosed {
		t.Fatalf("List err=%v", err)
	}
}
