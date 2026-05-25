package kafka

import (
	"context"
	"errors"
	"testing"

	qdriver "github.com/godx-jp/godx-platform-framework/queue/driver"
)

func TestRegisteredOnImport(t *testing.T) {
	if qdriver.Lookup(qdriver.DriverKafka) == nil {
		t.Fatal("kafka driver not registered")
	}
}

func TestNewValidatesBrokers(t *testing.T) {
	_, err := New(context.Background(), qdriver.Spec{Topic: "jobs"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewStubNotImplemented(t *testing.T) {
	_, err := New(context.Background(), qdriver.Spec{Brokers: []string{"localhost:9092"}, Topic: "jobs"})
	if !errors.Is(err, qdriver.ErrNotImplemented) {
		t.Fatalf("err=%v", err)
	}
}
