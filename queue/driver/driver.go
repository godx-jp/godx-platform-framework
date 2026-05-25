// Package driver is the public contract for queue backend implementations.
package driver

import (
	"context"
	"time"
)

// Job is a unit of work pulled from a queue backend.
type Job interface {
	ID() string
	Queue() string
	Payload() []byte
	Attempts() int
}

// Handler processes one Job. Returning an error causes the queue to emit
// job.failed and optionally requeue depending on backend policy.
type Handler func(ctx context.Context, job Job) error

// Backend is the in-process behaviour of one queue driver. Every method
// must be safe for concurrent use.
type Backend interface {
	Name() string

	// Push enqueues payload on queue. queue may be empty for the default
	// queue name configured on the Manager.
	Push(ctx context.Context, queue string, payload []byte) (Job, error)

	// Pop blocks until a job is available or ctx is canceled. Returns
	// (nil, nil) when the backend is idle and non-blocking pop is not
	// supported — memory driver uses a channel and respects ctx.
	Pop(ctx context.Context, queue string) (Job, error)

	// Delete removes a successfully processed job from the backend.
	Delete(ctx context.Context, job Job) error

	// Release returns a failed job to the queue for retry.
	Release(ctx context.Context, job Job, delay time.Duration) error

	Shutdown(ctx context.Context) error
}

// Constructor builds a Backend from a Spec.
type Constructor func(ctx context.Context, spec Spec) (Backend, error)
