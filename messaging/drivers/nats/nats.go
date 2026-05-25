package nats

import (
	"context"
	"fmt"

	mdriver "github.com/godx-jp/godx-platform-framework/messaging/driver"
)

func init() { mdriver.Register(mdriver.DriverNATS, New) }

func New(_ context.Context, spec mdriver.Spec) (mdriver.Broker, error) {
	if spec.NATSURL == "" {
		return nil, fmt.Errorf("messaging/nats: NATSURL required")
	}
	return nil, mdriver.ErrNotImplemented
}
