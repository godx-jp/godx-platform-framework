// Package log is a light mail transport that writes messages to
// the standard logger (slog). Useful for dev and tests.
package log

import (
	"context"
	"log/slog"
	"sync"

	mdriver "github.com/godx-jp/godx-platform-framework/mail/driver"
)

func init() {
	mdriver.Register(mdriver.DriverLog, func(_ context.Context, spec mdriver.Spec) (mdriver.Transport, error) {
		return New(spec.From), nil
	})
}

// previewLen is the number of leading body bytes included in the log
// preview when full-body logging is disabled (the default).
const previewLen = 64

type transport struct {
	from string

	// logBody, when true, emits the full message body to the log.
	// It defaults to false so the transport never leaks PII or
	// secrets (reset links, tokens) if selected outside dev.
	logBody bool

	mu     sync.Mutex
	closed bool
}

// New constructs a log Transport. from is recorded on each message
// when the Message.From field is empty.
//
// By default the full message body is NOT logged — only the recipient,
// subject, body length, and a short truncated preview are emitted, so
// that secrets in the body are not leaked to logs. Use NewWithBody to
// opt into full-body logging in trusted dev environments.
func New(from string) mdriver.Transport {
	return &transport{from: from}
}

// NewWithBody is like New but emits the full message body at Info level.
// Only use this in dev: the body may contain PII or secrets.
func NewWithBody(from string) mdriver.Transport {
	return &transport{from: from, logBody: true}
}

// preview returns the first previewLen bytes of body, marking it as
// truncated when the body is longer.
func preview(body string) string {
	if len(body) <= previewLen {
		return body
	}
	return body[:previewLen] + "…"
}

func (t *transport) Name() string { return mdriver.DriverLog }

func (t *transport) Send(_ context.Context, msg mdriver.Message) error {
	if err := t.checkOpen(); err != nil {
		return err
	}
	from := msg.From
	if from == "" {
		from = t.from
	}
	attrs := []any{
		"driver", mdriver.DriverLog,
		"from", from,
		"to", msg.To,
		"subject", msg.Subject,
		"body_bytes", len(msg.Body),
	}
	if t.logBody {
		attrs = append(attrs, "body", msg.Body)
	} else {
		attrs = append(attrs, "body_preview", preview(msg.Body))
	}
	slog.Info("mail", attrs...)
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
