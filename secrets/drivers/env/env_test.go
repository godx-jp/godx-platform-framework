package env

import (
	"context"
	"errors"
	"testing"

	sdriver "github.com/godx-jp/godx-platform-framework/secrets/driver"
)

func TestRegisteredOnImport(t *testing.T) {
	if sdriver.Lookup(sdriver.DriverEnv) == nil {
		t.Fatalf("env driver not registered on init")
	}
}

func TestGetDefaultPrefix(t *testing.T) {
	t.Setenv("SECRETS_DB_PASSWORD", "hunter2")
	s := New("")
	defer s.Shutdown(context.Background())
	v, err := s.Get(context.Background(), "db/password")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(v) != "hunter2" {
		t.Fatalf("value=%q", v)
	}
}

func TestGetCustomPrefix(t *testing.T) {
	t.Setenv("MYAPP_TOKEN", "abc")
	s := New("MYAPP_")
	defer s.Shutdown(context.Background())
	v, err := s.Get(context.Background(), "token")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(v) != "abc" {
		t.Fatalf("value=%q", v)
	}
}

func TestGetNoPrefix(t *testing.T) {
	t.Setenv("DB_PASS", "raw")
	s := New("-")
	defer s.Shutdown(context.Background())
	v, err := s.Get(context.Background(), "db/pass")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(v) != "raw" {
		t.Fatalf("value=%q", v)
	}
}

func TestKeyNormalisation(t *testing.T) {
	t.Setenv("SECRETS_AUTH_TOKEN", "ok")
	s := New("")
	defer s.Shutdown(context.Background())
	for _, k := range []string{"auth/token", "auth.token", "auth-token", "AUTH TOKEN"} {
		v, err := s.Get(context.Background(), k)
		if err != nil {
			t.Fatalf("Get(%q): %v", k, err)
		}
		if string(v) != "ok" {
			t.Fatalf("Get(%q)=%q", k, v)
		}
	}
}

func TestGetMissingReturnsNotFound(t *testing.T) {
	s := New("")
	defer s.Shutdown(context.Background())
	_, err := s.Get(context.Background(), "nope")
	if !errors.Is(err, sdriver.ErrNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestEmptyKey(t *testing.T) {
	s := New("")
	defer s.Shutdown(context.Background())
	_, err := s.Get(context.Background(), "")
	if !errors.Is(err, sdriver.ErrNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestWritesReadOnly(t *testing.T) {
	s := New("")
	defer s.Shutdown(context.Background())
	if err := s.Put(context.Background(), "k", []byte("v")); !errors.Is(err, sdriver.ErrReadOnly) {
		t.Fatalf("Put err=%v", err)
	}
	if err := s.Forget(context.Background(), "k"); !errors.Is(err, sdriver.ErrReadOnly) {
		t.Fatalf("Forget err=%v", err)
	}
}

func TestListNotSupported(t *testing.T) {
	s := New("")
	defer s.Shutdown(context.Background())
	keys, err := s.List(context.Background())
	if !errors.Is(err, sdriver.ErrListNotSupported) {
		t.Fatalf("List err=%v", err)
	}
	if keys != nil {
		t.Fatalf("keys=%v", keys)
	}
}

func TestAfterShutdown(t *testing.T) {
	s := New("")
	if err := s.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if _, err := s.Get(context.Background(), "anything"); !errors.Is(err, sdriver.ErrClosed) {
		t.Fatalf("Get err=%v", err)
	}
	if err := s.Put(context.Background(), "k", []byte("v")); !errors.Is(err, sdriver.ErrClosed) {
		t.Fatalf("Put err=%v", err)
	}
}

func TestShutdownIdempotent(t *testing.T) {
	s := New("")
	if err := s.Shutdown(context.Background()); err != nil {
		t.Fatalf("first: %v", err)
	}
	if err := s.Shutdown(context.Background()); err != nil {
		t.Fatalf("second: %v", err)
	}
}

func TestConstructorViaRegistry(t *testing.T) {
	t.Setenv("APP_FOO", "bar")
	s, err := sdriver.New(context.Background(), sdriver.Spec{
		Name:   sdriver.DriverEnv,
		Prefix: "APP_",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer s.Shutdown(context.Background())
	v, err := s.Get(context.Background(), "foo")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(v) != "bar" {
		t.Fatalf("v=%q", v)
	}
}
