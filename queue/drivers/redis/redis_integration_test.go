//go:build integration

package redis_test

import (
	"context"
	"errors"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/godx-jp/godx-platform-framework/events"
	"github.com/godx-jp/godx-platform-framework/queue"
	qdriver "github.com/godx-jp/godx-platform-framework/queue/driver"
	reddrv "github.com/godx-jp/godx-platform-framework/queue/drivers/redis"
)

func TestPushRetryDLQ(t *testing.T) {
	url := os.Getenv("REDIS_URL")
	if url == "" {
		url = "redis://127.0.0.1:6379"
	}

	ctx := context.Background()
	backend, err := reddrv.NewFromSpec(ctx, qdriver.Spec{
		Name:         qdriver.DriverRedis,
		URL:          url,
		Prefix:       "godx:queue:integration:",
		DefaultQueue: "default",
	})
	if err != nil {
		t.Skipf("Redis unavailable: %v", err)
	}
	defer func() { _ = backend.Shutdown(ctx) }()

	bus := events.New()
	var dead atomic.Int32
	bus.Listen(queue.EventDead, func(_ context.Context, _ events.Event) error {
		dead.Add(1)
		return nil
	})

	q := queue.NewQueue("integration", backend,
		queue.WithBus(bus),
		queue.WithRetryPolicy(queue.RetryPolicy{
			MaxAttempts: 2,
			DLQSuffix:   "-dlq",
		}),
	)

	const queueName = "retry-dlq"
	payload := []byte(`{"job":"integration"}`)

	if _, err := q.Push(ctx, queueName, payload); err != nil {
		t.Fatalf("Push: %v", err)
	}

	fail := func(_ context.Context, _ qdriver.Job) error {
		return errors.New("simulated handler failure")
	}

	if err := q.Dispatch(ctx, queueName, fail); err == nil {
		t.Fatal("expected first handler error")
	}

	// Release uses exponential backoff (default base 1s); wait before second attempt.
	time.Sleep(1500 * time.Millisecond)

	popCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := q.Dispatch(popCtx, queueName, fail); err == nil {
		t.Fatal("expected second handler error")
	}

	if dead.Load() != 1 {
		t.Fatalf("job.dead events=%d want 1", dead.Load())
	}

	dlqCtx, dlqCancel := context.WithTimeout(ctx, 2*time.Second)
	defer dlqCancel()
	job, err := backend.Pop(dlqCtx, queueName+"-dlq")
	if err != nil {
		t.Fatalf("Pop DLQ: %v", err)
	}
	if job == nil {
		t.Fatal("expected job in DLQ")
	}
	if string(job.Payload()) != string(payload) {
		t.Fatalf("DLQ payload=%q want %q", job.Payload(), payload)
	}
}
