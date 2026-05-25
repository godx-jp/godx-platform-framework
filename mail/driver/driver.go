// Package driver is the public contract for mail transport
// implementations.
package driver

import "context"

// Message is the unit passed to every transport.
type Message struct {
	From     string
	To       []string
	Subject  string
	Body     string
	HTMLBody string
	Headers  map[string]string
}

// Transport sends one Message. Implementations must be safe for
// concurrent use.
type Transport interface {
	Name() string
	Send(ctx context.Context, msg Message) error
	Shutdown(ctx context.Context) error
}

// Constructor builds a Transport from Spec.
type Constructor func(ctx context.Context, spec Spec) (Transport, error)
