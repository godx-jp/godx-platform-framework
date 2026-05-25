// Package kafka is a heavy stub for Apache Kafka. Registers the driver
// name for opt-in blank import.
//
//	import _ "github.com/godx-jp/godx-platform-framework/queue/drivers/kafka"
package kafka

import (
	"context"
	"fmt"

	qdriver "github.com/godx-jp/godx-platform-framework/queue/driver"
)

func init() {
	qdriver.Register(qdriver.DriverKafka, New)
}

func New(_ context.Context, spec qdriver.Spec) (qdriver.Backend, error) {
	if len(spec.Brokers) == 0 {
		return nil, fmt.Errorf("queue/kafka: Brokers required (set QUEUE_QUEUE_*_BROKERS)")
	}
	if spec.Topic == "" {
		return nil, fmt.Errorf("queue/kafka: Topic required (set QUEUE_QUEUE_*_TOPIC)")
	}
	return nil, qdriver.ErrNotImplemented
}
