// Package sendgrid sends mail via the SendGrid v3 REST API. Heavy —
// import via:
//
//	_ "github.com/godx-jp/godx-platform-framework/mail/drivers/sendgrid"
package sendgrid

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

const apiURL = "https://api.sendgrid.com/v3/mail/send"

func init() {
	mdriver.Register(mdriver.DriverSendGrid, func(_ context.Context, spec mdriver.Spec) (mdriver.Transport, error) {
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

// New constructs a SendGrid Transport. APIKey is required.
func New(spec mdriver.Spec) (mdriver.Transport, error) {
	if strings.TrimSpace(spec.APIKey) == "" {
		return nil, fmt.Errorf("mail/sendgrid: APIKey is required")
	}
	return &transport{
		apiKey: spec.APIKey,
		from:   spec.From,
		client: http.DefaultClient,
	}, nil
}

func (t *transport) Name() string { return mdriver.DriverSendGrid }

func (t *transport) Send(ctx context.Context, msg mdriver.Message) error {
	if err := t.checkOpen(); err != nil {
		return err
	}
	if len(msg.To) == 0 {
		return fmt.Errorf("mail/sendgrid: at least one recipient is required")
	}
	from := msg.From
	if from == "" {
		from = t.from
	}
	if from == "" {
		return fmt.Errorf("mail/sendgrid: from address is required")
	}
	bodyText := msg.Body
	contentType := "text/plain"
	if msg.HTMLBody != "" {
		bodyText = msg.HTMLBody
		contentType = "text/html"
	}
	payload := map[string]any{
		"personalizations": []map[string]any{{
			"to": toObjects(msg.To),
		}},
		"from": map[string]string{"email": from},
		"subject": msg.Subject,
		"content": []map[string]string{{
			"type":  contentType,
			"value": bodyText,
		}},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("mail/sendgrid: marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("mail/sendgrid: request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+t.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("mail/sendgrid: send: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("mail/sendgrid: api status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}

func toObjects(addrs []string) []map[string]string {
	out := make([]map[string]string, len(addrs))
	for i, a := range addrs {
		out[i] = map[string]string{"email": a}
	}
	return out
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
