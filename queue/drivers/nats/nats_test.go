package nats

import (
	"context"
	"errors"
	"testing"

	qdriver "github.com/godx-jp/godx-platform-framework/queue/driver"
)

func TestRegisteredOnImport(t *testing.T) {
	if qdriver.Lookup(qdriver.DriverNATS) == nil {
		t.Fatal("nats driver not registered")
	}
}

func TestNewRequiresURL(t *testing.T) {
	_, err := New(context.Background(), qdriver.Spec{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewStubNotImplemented(t *testing.T) {
	_, err := New(context.Background(), qdriver.Spec{NATSURL: "nats://127.0.0.1:4222"})
	if !errors.Is(err, qdriver.ErrNotImplemented) {
		t.Fatalf("err=%v", err)
	}
}
