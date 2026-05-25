package smtp_test

import (
	"context"
	"testing"

	mdriver "github.com/godx-jp/godx-platform-framework/mail/driver"
	_ "github.com/godx-jp/godx-platform-framework/mail/drivers/smtp"
)

func TestRegisteredOnImport(t *testing.T) {
	if mdriver.Lookup(mdriver.DriverSMTP) == nil {
		t.Fatal("smtp driver not registered")
	}
}

func TestConstructorValidatesHost(t *testing.T) {
	c := mdriver.Lookup(mdriver.DriverSMTP)
	if _, err := c(context.Background(), mdriver.Spec{Name: mdriver.DriverSMTP}); err == nil {
		t.Fatal("expected error for missing host")
	}
}

func TestConstructorDefaults(t *testing.T) {
	c := mdriver.Lookup(mdriver.DriverSMTP)
	tr, err := c(context.Background(), mdriver.Spec{
		Name: mdriver.DriverSMTP,
		Host: "localhost",
		From: "noreply@example.com",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if tr.Name() != mdriver.DriverSMTP {
		t.Fatalf("name=%q", tr.Name())
	}
	_ = tr.Shutdown(context.Background())
}

func TestSendRequiresRecipients(t *testing.T) {
	c := mdriver.Lookup(mdriver.DriverSMTP)
	tr, _ := c(context.Background(), mdriver.Spec{Name: mdriver.DriverSMTP, Host: "localhost", From: "a@b.c"})
	if err := tr.Send(context.Background(), mdriver.Message{}); err == nil {
		t.Fatal("expected error")
	}
}

func newTransport(t *testing.T) mdriver.Transport {
	t.Helper()
	c := mdriver.Lookup(mdriver.DriverSMTP)
	tr, err := c(context.Background(), mdriver.Spec{
		Name: mdriver.DriverSMTP,
		Host: "localhost",
		From: "noreply@example.com",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return tr
}

// TestSendRejectsSubjectHeaderInjection ensures a subject carrying an embedded
// CRLF + extra header is rejected before any SMTP dial happens.
func TestSendRejectsSubjectHeaderInjection(t *testing.T) {
	tr := newTransport(t)
	err := tr.Send(context.Background(), mdriver.Message{
		To:      []string{"user@example.com"},
		Subject: "Hello\r\nBcc: attacker@evil.com",
		Body:    "hi",
	})
	if err == nil {
		t.Fatal("expected header-injection subject to be rejected")
	}
}

// TestSendRejectsRecipientNewline ensures a recipient with an embedded newline
// is rejected.
func TestSendRejectsRecipientNewline(t *testing.T) {
	tr := newTransport(t)
	err := tr.Send(context.Background(), mdriver.Message{
		To:      []string{"user@example.com\nBcc: attacker@evil.com"},
		Subject: "Hello",
		Body:    "hi",
	})
	if err == nil {
		t.Fatal("expected newline recipient to be rejected")
	}
}

// TestSendRejectsInvalidFrom ensures a malformed From address is rejected.
func TestSendRejectsInvalidFrom(t *testing.T) {
	tr := newTransport(t)
	err := tr.Send(context.Background(), mdriver.Message{
		From:    "not an address",
		To:      []string{"user@example.com"},
		Subject: "Hello",
		Body:    "hi",
	})
	if err == nil {
		t.Fatal("expected invalid from address to be rejected")
	}
}
