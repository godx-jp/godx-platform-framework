package log_test

import (
	"context"
	"testing"

	mdriver "github.com/godx-jp/godx-platform-framework/mail/driver"
	_ "github.com/godx-jp/godx-platform-framework/mail/drivers/log"
)

func TestRegisteredOnImport(t *testing.T) {
	if mdriver.Lookup(mdriver.DriverLog) == nil {
		t.Fatal("log driver not registered")
	}
}

func TestSendAndShutdown(t *testing.T) {
	c := mdriver.Lookup(mdriver.DriverLog)
	tr, err := c(context.Background(), mdriver.Spec{Name: mdriver.DriverLog, From: "noreply@example.com"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := tr.Send(context.Background(), mdriver.Message{
		To:      []string{"user@example.com"},
		Subject: "Hi",
		Body:    "Hello",
	}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if err := tr.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if err := tr.Send(context.Background(), mdriver.Message{To: []string{"x@y.z"}}); err != mdriver.ErrClosed {
		t.Fatalf("err=%v", err)
	}
}
