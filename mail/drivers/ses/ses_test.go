package ses_test

import (
	"context"
	"testing"

	mdriver "github.com/godx-jp/godx-platform-framework/mail/driver"
	_ "github.com/godx-jp/godx-platform-framework/mail/drivers/ses"
)

func TestRegisteredOnImport(t *testing.T) {
	if mdriver.Lookup(mdriver.DriverSES) == nil {
		t.Fatal("ses driver not registered")
	}
}

func TestConstructorBuilds(t *testing.T) {
	c := mdriver.Lookup(mdriver.DriverSES)
	tr, err := c(context.Background(), mdriver.Spec{
		Name: mdriver.DriverSES,
		From: "noreply@example.com",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if tr.Name() != mdriver.DriverSES {
		t.Fatalf("name=%q", tr.Name())
	}
	_ = tr.Shutdown(context.Background())
}

func TestSendValidatesRecipients(t *testing.T) {
	c := mdriver.Lookup(mdriver.DriverSES)
	tr, _ := c(context.Background(), mdriver.Spec{Name: mdriver.DriverSES, From: "a@b.c"})
	if err := tr.Send(context.Background(), mdriver.Message{From: "a@b.c"}); err == nil {
		t.Fatal("expected error")
	}
}
