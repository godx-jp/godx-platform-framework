package queue

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/godx-jp/godx-platform-framework/events"
	qdriver "github.com/godx-jp/godx-platform-framework/queue/driver"
	"github.com/godx-jp/godx-platform-framework/queue/drivers/memory"
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

// failingBackend's Pop always returns an immediate error (simulating a
// backend that is down, e.g. Redis unreachable). It counts Pop calls.
type failingBackend struct {
	pops atomic.Int64
}

var errBackendDown = errors.New("backend down")

func (b *failingBackend) Name() string { return "failing" }
func (b *failingBackend) Push(context.Context, string, []byte) (qdriver.Job, error) {
	return nil, errBackendDown
}
func (b *failingBackend) Pop(context.Context, string) (qdriver.Job, error) {
	b.pops.Add(1)
	return nil, errBackendDown
}
func (b *failingBackend) Delete(context.Context, qdriver.Job) error                 { return nil }
func (b *failingBackend) Release(context.Context, qdriver.Job, time.Duration) error { return nil }
func (b *failingBackend) Shutdown(context.Context) error                            { return nil }

// TestRunBacksOffOnBackendError asserts that when Pop always errors
// immediately, the worker applies backoff (far fewer calls than an
// unbounded busy-spin would make) instead of hot-looping.
func TestRunBacksOffOnBackendError(t *testing.T) {
	be := &failingBackend{}
	q := NewQueue("default", be,
		WithWorkers(1),
		WithHandler(func(ctx context.Context, job qdriver.Job) error { return nil }),
	)
	ctx := context.Background()
	if err := q.Run(ctx, ""); err != nil {
		t.Fatal(err)
	}
	time.Sleep(500 * time.Millisecond)
	q.Stop()

	pops := be.pops.Load()
	// With base=100ms backoff doubling (100,200,400,...) inside a 500ms
	// window there should be only a handful of Pop attempts. A busy-spin
	// would make thousands. Use a generous ceiling to stay non-flaky.
	if pops == 0 {
		t.Fatalf("expected at least one Pop, got 0")
	}
	if pops > 30 {
		t.Fatalf("expected backoff to throttle Pop calls, got %d (busy-spin?)", pops)
	}
}

// TestRunStopPromptDuringBackoff asserts Stop returns promptly even while a
// worker is mid-backoff after a backend error (no multi-second hang).
func TestRunStopPromptDuringBackoff(t *testing.T) {
	be := &failingBackend{}
	q := NewQueue("default", be,
		WithWorkers(2),
		WithHandler(func(ctx context.Context, job qdriver.Job) error { return nil }),
	)
	ctx := context.Background()
	if err := q.Run(ctx, ""); err != nil {
		t.Fatal(err)
	}
	// Let workers hit an error and enter a backoff sleep.
	time.Sleep(150 * time.Millisecond)

	done := make(chan struct{})
	go func() {
		q.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop hung during backoff (sleep not interruptible)")
	}
}

// TestRunCtxCancelPromptDuringBackoff asserts ctx cancellation also exits
// workers promptly while mid-backoff.
func TestRunCtxCancelPromptDuringBackoff(t *testing.T) {
	be := &failingBackend{}
	q := NewQueue("default", be,
		WithWorkers(2),
		WithHandler(func(ctx context.Context, job qdriver.Job) error { return nil }),
	)
	ctx, cancel := context.WithCancel(context.Background())
	if err := q.Run(ctx, ""); err != nil {
		t.Fatal(err)
	}
	time.Sleep(150 * time.Millisecond)
	cancel()

	done := make(chan struct{})
	go func() {
		q.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("workers did not exit promptly on ctx cancel during backoff")
	}
}

// TestRunHappyPathBackToBack asserts the no-error path is unaffected:
// queued jobs are processed back-to-back with no injected delay.
func TestRunHappyPathBackToBack(t *testing.T) {
	var processed atomic.Int32
	q := NewQueue("default", newTestBackend(t),
		WithWorkers(2),
		WithHandler(func(ctx context.Context, job qdriver.Job) error {
			processed.Add(1)
			return nil
		}),
	)
	ctx := context.Background()
	const n = 12 // stay under the 16-slot memory channel capacity
	for i := 0; i < n; i++ {
		if _, err := q.Push(ctx, "", []byte("j")); err != nil {
			t.Fatal(err)
		}
	}
	if err := q.Run(ctx, ""); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(2 * time.Second)
	for processed.Load() < n {
		select {
		case <-deadline:
			q.Stop()
			t.Fatalf("processed=%d want %d (happy path not back-to-back?)", processed.Load(), n)
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}
	q.Stop()
}

var _ = memory.Name
