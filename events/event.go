package events

import (
	"context"
	"time"
)

// Event is the unit of dispatch. Name is the routing key; Payload is
// opaque to the bus and gets handed to listeners as-is. CreatedAt is
// stamped at dispatch time when the caller leaves it zero.
type Event struct {
	Name      string
	Payload   any
	Metadata  map[string]string
	CreatedAt time.Time
}

// Listener handles one Event. Returning an error causes Bus.Dispatch
// to include that error in its joined result; sibling listeners
// still run. Use ctx.Err to bail out early when the dispatch context
// is canceled.
type Listener func(ctx context.Context, e Event) error

// Subscription is a handle that Listen returns so callers can
// Cancel a specific listener without affecting siblings registered
// under the same pattern. Calling Cancel more than once is safe.
type Subscription interface {
	Cancel()
	Pattern() string
}
