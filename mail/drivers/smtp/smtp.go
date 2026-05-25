// Package smtp is a light mail transport using net/smtp.
package smtp

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"net/mail"
	"net/smtp"
	"strings"
	"sync"

	mdriver "github.com/godx-jp/godx-platform-framework/mail/driver"
)

const defaultPort = 587

// ErrHeaderInjection is returned when an address or subject contains CR/LF or
// other control characters that could smuggle extra SMTP headers.
var ErrHeaderInjection = errors.New("mail/smtp: header injection detected")

func init() {
	mdriver.Register(mdriver.DriverSMTP, func(_ context.Context, spec mdriver.Spec) (mdriver.Transport, error) {
		return New(spec)
	})
}

type transport struct {
	host     string
	port     int
	username string
	password string
	from     string

	mu     sync.Mutex
	closed bool
}

// New constructs an SMTP Transport from spec. Host is required.
func New(spec mdriver.Spec) (mdriver.Transport, error) {
	if strings.TrimSpace(spec.Host) == "" {
		return nil, fmt.Errorf("mail/smtp: host is required")
	}
	port := spec.Port
	if port == 0 {
		port = defaultPort
	}
	return &transport{
		host:     spec.Host,
		port:     port,
		username: spec.Username,
		password: spec.Password,
		from:     spec.From,
	}, nil
}

func (t *transport) Name() string { return mdriver.DriverSMTP }

func (t *transport) Send(_ context.Context, msg mdriver.Message) error {
	if err := t.checkOpen(); err != nil {
		return err
	}
	if len(msg.To) == 0 {
		return fmt.Errorf("mail/smtp: at least one recipient is required")
	}
	from := msg.From
	if from == "" {
		from = t.from
	}
	if from == "" {
		return fmt.Errorf("mail/smtp: from address is required")
	}
	if err := validateAddress(from); err != nil {
		return fmt.Errorf("mail/smtp: invalid from address %q: %w", from, err)
	}
	for _, rcpt := range msg.To {
		if err := validateAddress(rcpt); err != nil {
			return fmt.Errorf("mail/smtp: invalid recipient %q: %w", rcpt, err)
		}
	}
	if err := validateSubject(msg.Subject); err != nil {
		return fmt.Errorf("mail/smtp: invalid subject: %w", err)
	}
	body := msg.Body
	if msg.HTMLBody != "" {
		body = msg.HTMLBody
	}
	raw := buildMIME(from, msg.To, msg.Subject, body)
	addr := fmt.Sprintf("%s:%d", t.host, t.port)
	var auth smtp.Auth
	if t.username != "" {
		auth = smtp.PlainAuth("", t.username, t.password, t.host)
	}
	if err := smtp.SendMail(addr, auth, from, msg.To, []byte(raw)); err != nil {
		return fmt.Errorf("mail/smtp: send: %w", err)
	}
	return nil
}

func buildMIME(from string, to []string, subject, body string) string {
	var b strings.Builder
	b.WriteString("From: " + from + "\r\n")
	b.WriteString("To: " + strings.Join(to, ", ") + "\r\n")
	// Q-encode the subject: this both makes non-ASCII safe and guarantees any
	// stray CR/LF/control byte cannot break out into a new header line.
	b.WriteString("Subject: " + mime.QEncoding.Encode("utf-8", subject) + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	b.WriteString("\r\n")
	b.WriteString(body)
	return b.String()
}

// validateAddress rejects addresses that fail RFC 5322 parsing or that contain
// CR/LF (header-injection vectors). net/mail.ParseAddress accepts both bare
// addresses ("a@b.com") and display-name form ("Name <a@b.com>").
func validateAddress(addr string) error {
	if strings.ContainsAny(addr, "\r\n") {
		return ErrHeaderInjection
	}
	if _, err := mail.ParseAddress(addr); err != nil {
		return err
	}
	return nil
}

// validateSubject rejects subjects containing CR, LF, or other ASCII control
// characters that could inject additional headers. Note buildMIME also
// Q-encodes the subject as a second line of defence.
func validateSubject(subject string) error {
	for _, r := range subject {
		if r == '\r' || r == '\n' {
			return ErrHeaderInjection
		}
		if r < 0x20 || r == 0x7f {
			return ErrHeaderInjection
		}
	}
	return nil
}

func (t *transport) Shutdown(context.Context) error {
	t.mu.Lock()
	t.closed = true
	t.mu.Unlock()
	return nil
}

func (t *transport) checkOpen() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return mdriver.ErrClosed
	}
	return nil
}
