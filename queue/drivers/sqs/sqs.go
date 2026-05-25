// Package sqs is a heavy stub for AWS SQS. Registers the driver name so
// binaries can blank-import and configure QUEUE_QUEUE_*_DRIVER=sqs; full
// SQS integration ships in a follow-up release.
//
//	import _ "github.com/godx-jp/godx-platform-framework/queue/drivers/sqs"
package sqs

import (
	"context"
	"fmt"

	qdriver "github.com/godx-jp/godx-platform-framework/queue/driver"
)

func init() {
	qdriver.Register(qdriver.DriverSQS, New)
}

func New(_ context.Context, spec qdriver.Spec) (qdriver.Backend, error) {
	if spec.QueueURL == "" {
		return nil, fmt.Errorf("queue/sqs: QueueURL required (set QUEUE_QUEUE_*_URL)")
	}
	return nil, qdriver.ErrNotImplemented
}
