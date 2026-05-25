package driver

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestRegisterLookupRoundTrip(t *testing.T) {
	const name = "test-notify-driver-roundtrip"
	if Lookup(name) != nil {
		t.Fatalf("driver %q already registered", name)
	}
	Register(name, func(_ context.Context, _ Spec) (Channel, error) {
		return fakeChannel{name: name}, nil
	})
	defer func() {
		regMu.Lock()
		delete(reg, name)
		regMu.Unlock()
	}()
	c := Lookup(name)
	tr, err := c(context.Background(), Spec{Name: name})
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	if tr.Name() != name {
		t.Fatalf("name=%q", tr.Name())
	}
}

func TestNewUnknownDriver(t *testing.T) {
	_, err := New(context.Background(), Spec{Name: "nonexistent-notify-xyz"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not registered") {
		t.Fatalf("err=%v", err)
	}
}

type fakeChannel struct{ name string }

func (f fakeChannel) Name() string                                      { return f.name }
func (f fakeChannel) Send(context.Context, any, any) error              { return nil }
func (f fakeChannel) Shutdown(context.Context) error                    { return nil }

func TestDriverNameConstants(t *testing.T) {
	for _, n := range []string{DriverLog, DriverMail, DriverSlack, DriverDiscord, DriverWebhook, DriverDatabase} {
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
