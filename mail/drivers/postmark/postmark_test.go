package postmark_test

import (
	"context"
	"testing"

	mdriver "github.com/godx-jp/godx-platform-framework/mail/driver"
	_ "github.com/godx-jp/godx-platform-framework/mail/drivers/postmark"
)

func TestRegisteredOnImport(t *testing.T) {
	if mdriver.Lookup(mdriver.DriverPostmark) == nil {
		t.Fatal("postmark driver not registered")
	}
}

func TestConstructorValidatesAPIKey(t *testing.T) {
	c := mdriver.Lookup(mdriver.DriverPostmark)
	if _, err := c(context.Background(), mdriver.Spec{Name: mdriver.DriverPostmark}); err == nil {
		t.Fatal("expected error")
	}
}

func TestConstructorBuilds(t *testing.T) {
	c := mdriver.Lookup(mdriver.DriverPostmark)
	tr, err := c(context.Background(), mdriver.Spec{
		Name:   mdriver.DriverPostmark,
		APIKey: "token",
		From:   "noreply@example.com",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_ = tr.Shutdown(context.Background())
}
