package queue

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/godx-jp/godx-platform-framework/events"
	"github.com/godx-jp/godx-platform-framework/queue/drivers/memory"
	qdriver "github.com/godx-jp/godx-platform-framework/queue/driver"
)

func newTestBackend(t *testing.T) qdriver.Backend {
	t.Helper()
	b, err := qdriver.Lookup(qdriver.DriverMemory)(context.Background(), qdriver.Spec{QueueSize: 16})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestPushDispatch(t *testing.T) {
	bus := events.New()
	var processed atomic.Int32
	bus.Listen(EventProcessed, func(ctx context.Context, e events.Event) error {
		processed.Add(1)
		return nil
	})

	q := NewQueue("default", newTestBackend(t), WithBus(bus), WithHandler(func(ctx context.Context, job qdriver.Job) error {
		return nil
	}))

	ctx := context.Background()
	if _, err := q.Push(ctx, "", []byte("job")); err != nil {
		t.Fatal(err)
	}
	if err := q.Dispatch(ctx, "", nil); err != nil {
		t.Fatal(err)
	}
	if processed.Load() != 1 {
		t.Fatalf("processed=%d", processed.Load())
	}
}

func TestDispatchFailureEmitsFailed(t *testing.T) {
	bus := events.New()
	var failed atomic.Int32
	bus.Listen(EventFailed, func(ctx context.Context, e events.Event) error {
		failed.Add(1)
		return nil
	})

	q := NewQueue("default", newTestBackend(t), WithBus(bus))
	ctx := context.Background()
	_, _ = q.Push(ctx, "", []byte("x"))
	err := q.Dispatch(ctx, "", func(ctx context.Context, job qdriver.Job) error {
		return context.Canceled
	})
	if err == nil {
		t.Fatal("expected handler error")
	}
	if failed.Load() != 1 {
		t.Fatalf("failed events=%d", failed.Load())
	}
}

func TestRunWorkers(t *testing.T) {
	q := NewQueue("default", newTestBackend(t),
		WithWorkers(2),
		WithHandler(func(ctx context.Context, job qdriver.Job) error { return nil }),
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for i := 0; i < 4; i++ {
		_, _ = q.Push(ctx, "", []byte("j"))
	}
	if err := q.Run(ctx, ""); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	q.Stop()
}

func TestManagerDefault(t *testing.T) {
	mgr := NewManager()
	q := NewQueue("default", newTestBackend(t))
	if err := mgr.AddQueue(q); err != nil {
		t.Fatal(err)
	}
	got, err := mgr.Default()
	if err != nil {
		t.Fatal(err)
	}
	if got.Name() != "default" {
		t.Fatalf("name=%q", got.Name())
	}
}

var _ = memory.Name
