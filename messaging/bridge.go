package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/godx-jp/godx-platform-framework/events"
	"github.com/godx-jp/godx-platform-framework/messaging/envelope"
)

// BridgeOptions configures domain-event → integration publishing.
type BridgeOptions struct {
	// Prefix filters events by name prefix (empty publishes all).
	Prefix string
	// Source is the CloudEvents source URI.
	Source string
	// TypePrefix is prepended to event names for ce-type (default "com.godx.").
	TypePrefix string
}

// Bridge listens on an in-process events.Bus and publishes CloudEvents
// through a messaging Publisher.
type Bridge struct {
	bus events.Bus
	sub events.Subscription
}

// WireBridge registers a listener on bus that forwards matching events to pub.
func WireBridge(_ context.Context, bus events.Bus, pub *Publisher, opts BridgeOptions) (*Bridge, error) {
	if bus == nil || pub == nil {
		return nil, fmt.Errorf("messaging: WireBridge requires bus and publisher")
	}
	if opts.Source == "" {
		opts.Source = "godx://local"
	}
	if opts.TypePrefix == "" {
		opts.TypePrefix = "com.godx."
	}
	pattern := "*"
	if opts.Prefix != "" {
		pattern = opts.Prefix + "*"
	}
	b := &Bridge{bus: bus}
	b.sub = bus.Listen(pattern, func(c context.Context, e events.Event) error {
		if opts.Prefix != "" && !strings.HasPrefix(e.Name, opts.Prefix) {
			return nil
		}
		id := e.Name
		if !e.CreatedAt.IsZero() {
			id = fmt.Sprintf("%s-%d", e.Name, e.CreatedAt.UnixNano())
		}
		ce := envelope.Event{
			ID:      id,
			Type:    opts.TypePrefix + e.Name,
			Source:  opts.Source,
			Subject: e.Name,
			Data:    payloadBytes(e.Payload),
		}
		return pub.Publish(c, ce)
	})
	return b, nil
}

func payloadBytes(p any) []byte {
	switch v := p.(type) {
	case nil:
		return nil
	case []byte:
		return append([]byte(nil), v...)
	case string:
		return []byte(v)
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return []byte(fmt.Sprint(v))
		}
		return b
	}
}

// Close removes the bus listener.
func (b *Bridge) Close() {
	if b != nil && b.sub != nil {
		b.sub.Cancel()
		b.sub = nil
	}
}
