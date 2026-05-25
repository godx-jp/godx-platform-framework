package awssm

import (
	"context"
	"testing"

	sdriver "github.com/godx-jp/godx-platform-framework/secrets/driver"
)

func TestRegisteredOnImport(t *testing.T) {
	if sdriver.Lookup(sdriver.DriverAWSSM) == nil {
		t.Fatalf("awssm driver not registered")
	}
}

func TestSecretNameNormalisation(t *testing.T) {
	s := &store{prefix: "myapp"}
	for _, tc := range []struct{ in, want string }{
		{"db/password", "myapp/db/password"},
		{"/db/password", "myapp/db/password"},
		{"plain", "myapp/plain"},
	} {
		if got := s.secretName(tc.in); got != tc.want {
			t.Fatalf("secretName(%q)=%q want %q", tc.in, got, tc.want)
		}
	}

	s2 := &store{}
	if got := s2.secretName("/raw"); got != "raw" {
		t.Fatalf("no-prefix=%q", got)
	}
}

func TestShutdownIdempotent(t *testing.T) {
	s := &store{}
	if err := s.Shutdown(context.Background()); err != nil {
		t.Fatalf("first: %v", err)
	}
	if err := s.Shutdown(context.Background()); err != nil {
		t.Fatalf("second: %v", err)
	}
	if _, err := s.Get(context.Background(), "k"); err != sdriver.ErrClosed {
		t.Fatalf("Get err=%v", err)
	}
	if err := s.Put(context.Background(), "k", []byte("v")); err != sdriver.ErrClosed {
		t.Fatalf("Put err=%v", err)
	}
}

func TestEmptyKeys(t *testing.T) {
	s := &store{}
	if _, err := s.Get(context.Background(), ""); err != sdriver.ErrNotFound {
		t.Fatalf("Get empty=%v", err)
	}
	if err := s.Forget(context.Background(), ""); err != nil {
		t.Fatalf("Forget empty=%v", err)
	}
	if err := s.Put(context.Background(), "", []byte("v")); err == nil {
		t.Fatalf("Put empty should error")
	}
}

func TestConstructorRegistry(t *testing.T) {
	c := sdriver.Lookup(sdriver.DriverAWSSM)
	if c == nil {
		t.Fatal("nil constructor")
	}
	// Don't actually construct — that requires AWS creds. Just confirm
	// the constructor is callable and rejects nothing about empty spec
	// (no required fields client-side).
}
