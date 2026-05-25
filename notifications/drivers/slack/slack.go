// Package slack posts notifications to Slack incoming webhooks.
package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	ndriver "github.com/godx-jp/godx-platform-framework/notifications/driver"
	"github.com/godx-jp/godx-platform-framework/notifications/contract"
)

func init() {
	ndriver.Register(ndriver.DriverSlack, func(_ context.Context, spec ndriver.Spec) (ndriver.Channel, error) {
		return New(spec.WebhookURL, http.DefaultClient), nil
	})
}

type channel struct {
	defaultURL string
	client     ndriver.HTTPDoer

	mu     sync.Mutex
	closed bool
}

// New constructs a Slack channel. defaultURL is used when the
// notification does not override the webhook URL.
func New(defaultURL string, client ndriver.HTTPDoer) ndriver.Channel {
	if client == nil {
		client = http.DefaultClient
	}
	return &channel{defaultURL: defaultURL, client: client}
}

func (c *channel) Name() string { return ndriver.DriverSlack }

func (c *channel) Send(ctx context.Context, notifiable, notification any) error {
	if err := c.checkOpen(); err != nil {
		return err
	}
	n, ok := notifiable.(contract.Notifiable)
	if !ok {
		return fmt.Errorf("notifications/slack: notifiable must implement contract.Notifiable")
	}
	p, ok := notification.(contract.SlackPresenter)
	if !ok {
		return fmt.Errorf("notifications/slack: notification must implement SlackPresenter")
	}
	msg := p.ToSlack(n)
	url := msg.WebhookURL
	if url == "" {
		url = n.RouteNotificationFor(ndriver.DriverSlack)
	}
	if url == "" {
		url = c.defaultURL
	}
	if url == "" {
		return fmt.Errorf("notifications/slack: webhook URL is required")
	}
	body, err := json.Marshal(map[string]string{"text": msg.Text})
	if err != nil {
		return fmt.Errorf("notifications/slack: marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("notifications/slack: post: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("notifications/slack: status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
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
