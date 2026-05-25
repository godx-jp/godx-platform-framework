package kafka

import (
	"context"
	"fmt"

	mdriver "github.com/godx-jp/godx-platform-framework/messaging/driver"
)

func init() { mdriver.Register(mdriver.DriverKafka, New) }

func New(_ context.Context, spec mdriver.Spec) (mdriver.Broker, error) {
	if len(spec.KafkaBrokers) == 0 {
		return nil, fmt.Errorf("messaging/kafka: brokers required")
	}
	return nil, mdriver.ErrNotImplemented
}
