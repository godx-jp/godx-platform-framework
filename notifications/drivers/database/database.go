// Package database persists notifications via a caller-provided store.
package database

import (
	"context"
	"fmt"
	"sync"

	ndriver "github.com/godx-jp/godx-platform-framework/notifications/driver"
	"github.com/godx-jp/godx-platform-framework/notifications/contract"
)

// Channel writes notification rows through DatabaseStore.
type Channel struct {
	store ndriver.DatabaseStore

	mu     sync.Mutex
	closed bool
}

// New constructs a database notification channel.
func New(store ndriver.DatabaseStore) ndriver.Channel {
	return &Channel{store: store}
}

func (c *Channel) Name() string { return ndriver.DriverDatabase }

func (c *Channel) Send(ctx context.Context, notifiable, notification any) error {
	if err := c.checkOpen(); err != nil {
		return err
	}
	if c.store == nil {
		return fmt.Errorf("notifications/database: DatabaseStore is required")
	}
	n, ok := notifiable.(contract.Notifiable)
	if !ok {
		return fmt.Errorf("notifications/database: notifiable must implement contract.Notifiable")
	}
	p, ok := notification.(contract.DatabasePresenter)
	if !ok {
		return fmt.Errorf("notifications/database: notification must implement DatabasePresenter")
	}
	msg := p.ToDatabase(n)
	rec := ndriver.DatabaseRecord{
		Channel: ndriver.DriverDatabase,
		Type:    msg.Type,
		Data:    msg.Data,
	}
	if id, ok := notifiable.(contract.IdentifiableNotifiable); ok {
		rec.NotifiableType = id.NotifiableType()
		rec.NotifiableID = id.NotifiableID()
	}
	return c.store.Store(ctx, rec)
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
