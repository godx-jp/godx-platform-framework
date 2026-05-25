package sqs

import (
	"context"
	"errors"
	"testing"

	qdriver "github.com/godx-jp/godx-platform-framework/queue/driver"
)

func TestRegisteredOnImport(t *testing.T) {
	if qdriver.Lookup(qdriver.DriverSQS) == nil {
		t.Fatal("sqs driver not registered")
	}
}

func TestNewRequiresQueueURL(t *testing.T) {
	_, err := New(context.Background(), qdriver.Spec{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewStubNotImplemented(t *testing.T) {
	_, err := New(context.Background(), qdriver.Spec{QueueURL: "https://sqs.example/queue"})
	if !errors.Is(err, qdriver.ErrNotImplemented) {
		t.Fatalf("err=%v", err)
	}
}
