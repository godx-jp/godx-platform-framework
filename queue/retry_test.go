package queue

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/godx-jp/godx-platform-framework/events"
	qdriver "github.com/godx-jp/godx-platform-framework/queue/driver"
)

func TestMaxAttemptsMovesJobToDLQ(t *testing.T) {
	bus := events.New()
	var dead atomic.Int32
	bus.Listen(EventDead, func(ctx context.Context, e events.Event) error {
		dead.Add(1)
		return nil
	})

	q := NewQueue("default", newTestBackend(t),
		WithBus(bus),
		WithRetryPolicy(RetryPolicy{MaxAttempts: 1, DLQSuffix: "-dlq"}),
	)
	ctx := context.Background()
	_, _ = q.Push(ctx, "emails", []byte("x"))
	err := q.Dispatch(ctx, "emails", func(ctx context.Context, job qdriver.Job) error {
		return context.Canceled
	})
	if err == nil {
		t.Fatal("expected handler error")
	}
	if dead.Load() != 1 {
		t.Fatalf("dead events=%d", dead.Load())
	}
}
