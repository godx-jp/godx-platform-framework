// Package log is a light notification channel that writes payloads to slog.
package log

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	ndriver "github.com/godx-jp/godx-platform-framework/notifications/driver"
	"github.com/godx-jp/godx-platform-framework/notifications/contract"
)

func init() {
	ndriver.Register(ndriver.DriverLog, func(_ context.Context, _ ndriver.Spec) (ndriver.Channel, error) {
		return New(), nil
	})
}

type channel struct {
	mu     sync.Mutex
	closed bool
}

// New constructs a log notification channel.
func New() ndriver.Channel { return &channel{} }

func (c *channel) Name() string { return ndriver.DriverLog }

func (c *channel) Send(_ context.Context, notifiable, notification any) error {
	if err := c.checkOpen(); err != nil {
		return err
	}
	n, ok := notifiable.(contract.Notifiable)
	if !ok {
		return fmt.Errorf("notifications/log: notifiable must implement contract.Notifiable")
	}
	note, ok := notification.(contract.Notification)
	if !ok {
		return fmt.Errorf("notifications/log: notification must implement contract.Notification")
	}
	channels := note.Via(n)
	slog.Info("notification",
		"driver", ndriver.DriverLog,
		"channels", channels,
		"route", n.RouteNotificationFor(ndriver.DriverLog),
	)
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
