// Package webhook posts generic HTTP webhook notifications.
package webhook

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/godx-jp/godx-platform-framework/notifications/contract"
	ndriver "github.com/godx-jp/godx-platform-framework/notifications/driver"
	"github.com/godx-jp/godx-platform-framework/notifications/internal/urlguard"
)

func init() {
	ndriver.Register(ndriver.DriverWebhook, func(_ context.Context, spec ndriver.Spec) (ndriver.Channel, error) {
		return New(spec.WebhookURL, http.DefaultClient), nil
	})
}

type channel struct {
	defaultURL string
	client     ndriver.HTTPDoer

	mu     sync.Mutex
	closed bool
}

// New constructs a webhook channel.
func New(defaultURL string, client ndriver.HTTPDoer) ndriver.Channel {
	if client == nil {
		client = http.DefaultClient
	}
	return &channel{defaultURL: defaultURL, client: client}
}

func (c *channel) Name() string { return ndriver.DriverWebhook }

func (c *channel) Send(ctx context.Context, notifiable, notification any) error {
	if err := c.checkOpen(); err != nil {
		return err
	}
	n, ok := notifiable.(contract.Notifiable)
	if !ok {
		return fmt.Errorf("notifications/webhook: notifiable must implement contract.Notifiable")
	}
	p, ok := notification.(contract.WebhookPresenter)
	if !ok {
		return fmt.Errorf("notifications/webhook: notification must implement WebhookPresenter")
	}
	msg := p.ToWebhook(n)
	url := msg.URL
	if url == "" {
		url = n.RouteNotificationFor(ndriver.DriverWebhook)
	}
	if url == "" {
		url = c.defaultURL
	}
	if url == "" {
		return fmt.Errorf("notifications/webhook: URL is required")
	}
	// SECURITY: destination URL is attacker-influenced; validate against the
	// SSRF policy before issuing any request. Redirects are not followed by
	// the injected client; if that changes, the redirect target must be
	// re-validated via urlguard.Validate.
	if _, err := urlguard.Validate(url); err != nil {
		return fmt.Errorf("notifications/webhook: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(msg.Body))
	if err != nil {
		return err
	}
	for k, v := range msg.Headers {
		req.Header.Set(k, v)
	}
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("notifications/webhook: post: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("notifications/webhook: status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}

func (c *channel) Shutdown(context.Context) error {
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()
	return nil
}

func (c *channel) checkOpen() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return ndriver.ErrClosed
	}
	return nil
}

// SetHTTPClient replaces the default client on a channel built via
// New. Used by notifications.Module to inject httpclient.
func SetHTTPClient(ch ndriver.Channel, client ndriver.HTTPDoer) {
	if wc, ok := ch.(*channel); ok && client != nil {
		wc.client = client
	}
}
