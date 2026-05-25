package sendgrid_test

import (
	"context"
	"testing"

	mdriver "github.com/godx-jp/godx-platform-framework/mail/driver"
	_ "github.com/godx-jp/godx-platform-framework/mail/drivers/sendgrid"
)

func TestRegisteredOnImport(t *testing.T) {
	if mdriver.Lookup(mdriver.DriverSendGrid) == nil {
		t.Fatal("sendgrid driver not registered")
	}
}

func TestConstructorValidatesAPIKey(t *testing.T) {
	c := mdriver.Lookup(mdriver.DriverSendGrid)
	if _, err := c(context.Background(), mdriver.Spec{Name: mdriver.DriverSendGrid}); err == nil {
		t.Fatal("expected error")
	}
}

func TestConstructorBuilds(t *testing.T) {
	c := mdriver.Lookup(mdriver.DriverSendGrid)
	tr, err := c(context.Background(), mdriver.Spec{
		Name:   mdriver.DriverSendGrid,
		APIKey: "SG.test",
		From:   "noreply@example.com",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_ = tr.Shutdown(context.Background())
}
