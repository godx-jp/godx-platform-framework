// Package nats is a heavy stub for NATS JetStream. Registers the driver
// name for opt-in blank import.
//
//	import _ "github.com/godx-jp/godx-platform-framework/queue/drivers/nats"
package nats

import (
	"context"
	"fmt"

	qdriver "github.com/godx-jp/godx-platform-framework/queue/driver"
)

func init() {
	qdriver.Register(qdriver.DriverNATS, New)
}

func New(_ context.Context, spec qdriver.Spec) (qdriver.Backend, error) {
	if spec.NATSURL == "" {
		return nil, fmt.Errorf("queue/nats: NATSURL required (set QUEUE_QUEUE_*_NATS_URL)")
	}
	return nil, qdriver.ErrNotImplemented
}
