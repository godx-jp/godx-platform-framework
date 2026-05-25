// Package driver is the public contract for notification channel
// implementations.
package driver

import (
	"context"
	"net/http"
)

// Channel delivers a Notification to one backend. notifiable and
// notification are the notifications package interfaces (passed as
// any to avoid an import cycle).
type Channel interface {
	Name() string
	Send(ctx context.Context, notifiable, notification any) error
	Shutdown(ctx context.Context) error
}

// Constructor builds a Channel from Spec.
type Constructor func(ctx context.Context, spec Spec) (Channel, error)

// DatabaseStore persists in-app notifications. Callers provide their
// own table/repository implementation.
type DatabaseStore interface {
	Store(ctx context.Context, record DatabaseRecord) error
}

// DatabaseRecord is written by the database channel driver.
type DatabaseRecord struct {
	NotifiableType string
	NotifiableID   string
	Channel        string
	Type           string
	Data           []byte
}

// HTTPDoer performs outbound HTTP for webhook-style channels.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}
