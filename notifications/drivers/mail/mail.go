// Package mail delivers notifications via the mail module.
package mail

import (
	"context"
	"fmt"
	"sync"

	"github.com/godx-jp/godx-platform-framework/mail"
	ndriver "github.com/godx-jp/godx-platform-framework/notifications/driver"
	"github.com/godx-jp/godx-platform-framework/notifications/contract"
)

// Channel sends mail notifications using a mail.Manager.
type Channel struct {
	mgr    *mail.Manager
	mailer string

	mu     sync.Mutex
	closed bool
}

// New constructs a mail notification channel. mailer names the
// transport on mgr (empty uses the default).
func New(mgr *mail.Manager, mailer string) ndriver.Channel {
	return &Channel{mgr: mgr, mailer: mailer}
}

func (c *Channel) Name() string { return ndriver.DriverMail }

func (c *Channel) Send(ctx context.Context, notifiable, notification any) error {
	if err := c.checkOpen(); err != nil {
		return err
	}
	if c.mgr == nil {
		return fmt.Errorf("notifications/mail: mail manager is required")
	}
	n, ok := notifiable.(contract.Notifiable)
	if !ok {
		return fmt.Errorf("notifications/mail: notifiable must implement contract.Notifiable")
	}
	p, ok := notification.(contract.MailPresenter)
	if !ok {
		return fmt.Errorf("notifications/mail: notification must implement MailPresenter")
	}
	msg := p.ToMail(n)
	to := msg.To
	if len(to) == 0 {
		if addr := n.RouteNotificationFor(ndriver.DriverMail); addr != "" {
			to = []string{addr}
		}
	}
	if len(to) == 0 {
		return fmt.Errorf("notifications/mail: no recipients")
	}
	ml, err := c.mgr.Mailer(c.mailer)
	if err != nil {
		return err
	}
	builder := ml.To(to...).Subject(msg.Subject).Body(msg.Body)
	if msg.HTML != "" {
		builder = builder.HTML(msg.HTML)
	}
	return builder.Send(ctx)
}

func (c *Channel) Shutdown(context.Context) error {
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()
	return nil
}

func (c *Channel) checkOpen() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return ndriver.ErrClosed
	}
	return nil
}
