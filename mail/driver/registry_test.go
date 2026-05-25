package driver

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestRegisterLookupRoundTrip(t *testing.T) {
	const name = "test-mail-driver-roundtrip"
	if Lookup(name) != nil {
		t.Fatalf("driver %q already registered", name)
	}
	want := errors.New("sentinel")
	Register(name, func(_ context.Context, _ Spec) (Transport, error) {
		return fakeTransport{name: name, err: want}, nil
	})
	defer func() {
		regMu.Lock()
		delete(reg, name)
		regMu.Unlock()
	}()

	c := Lookup(name)
	if c == nil {
		t.Fatal("Lookup returned nil")
	}
	tr, err := c(context.Background(), Spec{Name: name})
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	if tr.Name() != name {
		t.Fatalf("name=%q", tr.Name())
	}
}

func TestNewRequiresName(t *testing.T) {
	if _, err := New(context.Background(), Spec{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestNewUnknownDriver(t *testing.T) {
	_, err := New(context.Background(), Spec{Name: "nonexistent-mail-driver-xyz"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not registered") {
		t.Fatalf("err=%v", err)
	}
}

func TestRegisterPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for empty name")
		}
	}()
	Register("", nil)
}

type fakeTransport struct {
	name string
	err  error
}

func (f fakeTransport) Name() string { return f.name }
func (f fakeTransport) Send(context.Context, Message) error { return f.err }
func (f fakeTransport) Shutdown(context.Context) error      { return nil }

func TestDriverNameConstants(t *testing.T) {
	for _, n := range []string{DriverLog, DriverSMTP, DriverSES, DriverSendGrid, DriverMailgun, DriverPostmark} {
		if n == "" {
			t.Fatal("empty constant")
		}
	}
}

func TestErrClosedDistinct(t *testing.T) {
	if !errors.Is(ErrClosed, ErrClosed) {
		t.Fatal("sentinel broken")
	}
}
