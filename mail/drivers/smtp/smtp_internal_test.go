package smtp

import (
	"errors"
	"net/mail"
	"strings"
	"testing"
)

// TestBuildMIMEValid checks the happy path: a normal subject/body produce a
// well-formed message whose headers parse and whose body is preserved.
func TestBuildMIMEValid(t *testing.T) {
	raw := buildMIME(
		"noreply@example.com",
		[]string{"a@b.com", "Name <c@d.com>"},
		"Order shipped",
		"Your order is on the way.",
	)

	headerPart, body, ok := strings.Cut(raw, "\r\n\r\n")
	if !ok {
		t.Fatalf("message has no header/body separator:\n%q", raw)
	}
	if body != "Your order is on the way." {
		t.Fatalf("body=%q", body)
	}

	msg, err := mail.ReadMessage(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if got := msg.Header.Get("From"); got != "noreply@example.com" {
		t.Fatalf("From=%q", got)
	}
	if got := msg.Header.Get("To"); got != "a@b.com, Name <c@d.com>" {
		t.Fatalf("To=%q", got)
	}
	if got := msg.Header.Get("Subject"); got != "Order shipped" {
		t.Fatalf("Subject=%q (decoded from %q)", got, headerPart)
	}
	if got := msg.Header.Get("Content-Type"); !strings.Contains(got, "text/plain") {
		t.Fatalf("Content-Type=%q", got)
	}
}

// TestBuildMIMESubjectQEncoded ensures a CRLF-bearing subject cannot break out
// into a new header line even if it reached buildMIME — the Q-encoding folds it
// into the Subject value so mail.ReadMessage sees exactly one Subject header.
func TestBuildMIMESubjectQEncoded(t *testing.T) {
	raw := buildMIME(
		"noreply@example.com",
		[]string{"a@b.com"},
		"Hi\r\nBcc: attacker@evil.com",
		"body",
	)
	msg, err := mail.ReadMessage(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if _, ok := msg.Header["Bcc"]; ok {
		t.Fatal("CRLF subject smuggled a Bcc header")
	}
	// The raw on-the-wire header block must not contain an actual CR or LF
	// inside the Subject value (other than the trailing header terminator).
	headerBlock, _, _ := strings.Cut(raw, "\r\n\r\n")
	for _, line := range strings.Split(headerBlock, "\r\n") {
		if strings.HasPrefix(line, "Subject:") {
			if strings.ContainsAny(strings.TrimPrefix(line, "Subject:"), "\r\n") {
				t.Fatalf("Subject header contains a raw CR/LF: %q", line)
			}
		}
	}
	// And the decoded value, while it may echo the attacker's text, must not be
	// split across header lines: there is exactly one Subject header.
	if len(msg.Header["Subject"]) != 1 {
		t.Fatalf("expected exactly one Subject header, got %v", msg.Header["Subject"])
	}
}

func TestValidateAddress(t *testing.T) {
	good := []string{"a@b.com", "Name <a@b.com>", "  spaced@b.com  "}
	for _, a := range good {
		if err := validateAddress(a); err != nil {
			t.Errorf("validateAddress(%q) = %v, want nil", a, err)
		}
	}

	if err := validateAddress("a@b.com\r\nBcc: x@y.com"); !errors.Is(err, ErrHeaderInjection) {
		t.Errorf("CRLF address: got %v, want ErrHeaderInjection", err)
	}
	if err := validateAddress("a@b.com\nx"); !errors.Is(err, ErrHeaderInjection) {
		t.Errorf("LF address: got %v, want ErrHeaderInjection", err)
	}
	if err := validateAddress("not an address"); err == nil {
		t.Error("expected parse error for malformed address")
	}
}

func TestValidateSubject(t *testing.T) {
	if err := validateSubject("Perfectly normal subject 123"); err != nil {
		t.Errorf("valid subject rejected: %v", err)
	}
	if err := validateSubject("Hi\r\nBcc: x@y.com"); !errors.Is(err, ErrHeaderInjection) {
		t.Errorf("CRLF subject: got %v, want ErrHeaderInjection", err)
	}
	if err := validateSubject("Tab\there"); !errors.Is(err, ErrHeaderInjection) {
		t.Errorf("control char subject: got %v, want ErrHeaderInjection", err)
	}
}
