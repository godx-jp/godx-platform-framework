// Package postmark sends mail via the Postmark REST API. Heavy — import via:
//
//	_ "github.com/godx-jp/godx-platform-framework/mail/drivers/postmark"
package postmark

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	mdriver "github.com/godx-jp/godx-platform-framework/mail/driver"
)

const apiURL = "https://api.postmarkapp.com/email"

func init() {
	mdriver.Register(mdriver.DriverPostmark, func(_ context.Context, spec mdriver.Spec) (mdriver.Transport, error) {
		return New(spec)
	})
}

type transport struct {
	apiKey string
	from   string
	client *http.Client

	mu     sync.Mutex
	closed bool
}

// New constructs a Postmark Transport. APIKey is required.
func New(spec mdriver.Spec) (mdriver.Transport, error) {
	if strings.TrimSpace(spec.APIKey) == "" {
		return nil, fmt.Errorf("mail/postmark: APIKey is required")
	}
	return &transport{
		apiKey: spec.APIKey,
		from:   spec.From,
		client: http.DefaultClient,
	}, nil
}

func (t *transport) Name() string { return mdriver.DriverPostmark }

func (t *transport) Send(ctx context.Context, msg mdriver.Message) error {
	if err := t.checkOpen(); err != nil {
		return err
	}
	if len(msg.To) == 0 {
		return fmt.Errorf("mail/postmark: at least one recipient is required")
	}
	from := msg.From
	if from == "" {
		from = t.from
	}
	if from == "" {
		return fmt.Errorf("mail/postmark: from address is required")
	}
	payload := map[string]any{
		"From":     from,
		"To":       strings.Join(msg.To, ","),
		"Subject":  msg.Subject,
		"TextBody": msg.Body,
	}
	if msg.HTMLBody != "" {
		payload["HtmlBody"] = msg.HTMLBody
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("mail/postmark: marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("mail/postmark: request: %w", err)
	}
	req.Header.Set("X-Postmark-Server-Token", t.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("mail/postmark: send: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("mail/postmark: api status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
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
