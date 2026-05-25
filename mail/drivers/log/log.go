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

type transport struct {
	from string

	mu     sync.Mutex
	closed bool
}

// New constructs a log Transport. from is recorded on each message
// when the Message.From field is empty.
func New(from string) mdriver.Transport {
	return &transport{from: from}
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
	slog.Info("mail",
		"driver", mdriver.DriverLog,
		"from", from,
		"to", msg.To,
		"subject", msg.Subject,
		"body", msg.Body,
	)
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
