// Package mailgun sends mail via the Mailgun REST API. Heavy — import via:
//
//	_ "github.com/godx-jp/godx-platform-framework/mail/drivers/mailgun"
package mailgun

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	mdriver "github.com/godx-jp/godx-platform-framework/mail/driver"
)

func init() {
	mdriver.Register(mdriver.DriverMailgun, func(_ context.Context, spec mdriver.Spec) (mdriver.Transport, error) {
		return New(spec)
	})
}

type transport struct {
	apiKey string
	domain string
	from   string
	client *http.Client

	mu     sync.Mutex
	closed bool
}

// New constructs a Mailgun Transport. APIKey and Domain are required.
func New(spec mdriver.Spec) (mdriver.Transport, error) {
	if strings.TrimSpace(spec.APIKey) == "" {
		return nil, fmt.Errorf("mail/mailgun: APIKey is required")
	}
	if strings.TrimSpace(spec.Domain) == "" {
		return nil, fmt.Errorf("mail/mailgun: Domain is required")
	}
	return &transport{
		apiKey: spec.APIKey,
		domain: spec.Domain,
		from:   spec.From,
		client: http.DefaultClient,
	}, nil
}

func (t *transport) Name() string { return mdriver.DriverMailgun }

func (t *transport) Send(ctx context.Context, msg mdriver.Message) error {
	if err := t.checkOpen(); err != nil {
		return err
	}
	if len(msg.To) == 0 {
		return fmt.Errorf("mail/mailgun: at least one recipient is required")
	}
	from := msg.From
	if from == "" {
		from = t.from
	}
	if from == "" {
		return fmt.Errorf("mail/mailgun: from address is required")
	}
	endpoint := fmt.Sprintf("https://api.mailgun.net/v3/%s/messages", t.domain)
	form := url.Values{}
	form.Set("from", from)
	form.Set("to", strings.Join(msg.To, ","))
	form.Set("subject", msg.Subject)
	if msg.HTMLBody != "" {
		form.Set("html", msg.HTMLBody)
	} else {
		form.Set("text", msg.Body)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("mail/mailgun: request: %w", err)
	}
	req.SetBasicAuth("api", t.apiKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("mail/mailgun: send: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("mail/mailgun: api status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
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
