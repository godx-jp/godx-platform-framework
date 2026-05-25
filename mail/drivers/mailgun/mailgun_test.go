package mailgun_test

import (
	"context"
	"testing"

	mdriver "github.com/godx-jp/godx-platform-framework/mail/driver"
	_ "github.com/godx-jp/godx-platform-framework/mail/drivers/mailgun"
)

func TestRegisteredOnImport(t *testing.T) {
	if mdriver.Lookup(mdriver.DriverMailgun) == nil {
		t.Fatal("mailgun driver not registered")
	}
}

func TestConstructorValidates(t *testing.T) {
	c := mdriver.Lookup(mdriver.DriverMailgun)
	if _, err := c(context.Background(), mdriver.Spec{Name: mdriver.DriverMailgun, APIKey: "k"}); err == nil {
		t.Fatal("expected domain error")
	}
	if _, err := c(context.Background(), mdriver.Spec{Name: mdriver.DriverMailgun, Domain: "mg.example.com"}); err == nil {
		t.Fatal("expected api key error")
	}
}

func TestConstructorBuilds(t *testing.T) {
	c := mdriver.Lookup(mdriver.DriverMailgun)
	tr, err := c(context.Background(), mdriver.Spec{
		Name:   mdriver.DriverMailgun,
		APIKey: "key",
		Domain: "mg.example.com",
		From:   "noreply@example.com",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_ = tr.Shutdown(context.Background())
}
