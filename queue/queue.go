package queue

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/godx-jp/godx-platform-framework/events"
	qdriver "github.com/godx-jp/godx-platform-framework/queue/driver"
)

const (
	EventProcessing = "job.processing"
	EventProcessed  = "job.processed"
	EventFailed     = "job.failed"
	EventDead       = "job.dead"
)

const (
	// runPopTimeout bounds how long a single Run worker waits for a job
	// before looping; an idle (empty-queue) wait is paced by this alone.
	runPopTimeout = 100 * time.Millisecond
	// runErrBackoffBase / runErrBackoffMax bound the exponential backoff a
	// Run worker applies after a real backend error (e.g. Redis down) to
	// avoid busy-spinning a failing backend. The delay starts at base,
	// doubles per consecutive error, caps at max, and resets on the next
	// non-error cycle.
	runErrBackoffBase = 100 * time.Millisecond
	runErrBackoffMax  = 5 * time.Second
)

// Queue is the user-facing handle for one named queue connection.
type Queue struct {
	name         string
	defaultQueue string
	backend      qdriver.Backend
	bus          events.Bus
	handler      qdriver.Handler
	retry        RetryPolicy

	mu      sync.RWMutex
	workers int
	running bool
	stop    chan struct{}
	wg      sync.WaitGroup
}

// NewQueue wraps an already-constructed backend. Prefer queue.Module for
// the normal boot path.
func NewQueue(name string, backend qdriver.Backend, opts ...Option) *Queue {
	q := &Queue{
		name:         name,
		defaultQueue: "default",
		backend:      backend,
		workers:      1,
		retry:        defaultRetryPolicy(),
	}
	for _, o := range opts {
		o(q)
	}
	return q
}

// Option configures a Queue.
type Option func(*Queue)

// WithBus attaches an events.Bus for job.processing / job.processed /
// job.failed lifecycle hooks.
func WithBus(bus events.Bus) Option {
	return func(q *Queue) { q.bus = bus }
}

// WithHandler sets the default handler used by Run.
func WithHandler(h qdriver.Handler) Option {
	return func(q *Queue) { q.handler = h }
}

// WithDefaultQueue sets the queue name used when Push receives "".
func WithDefaultQueue(name string) Option {
	return func(q *Queue) {
		if name != "" {
			q.defaultQueue = name
		}
	}
}

// WithRetryPolicy configures retry, backoff, and DLQ routing.
func WithRetryPolicy(p RetryPolicy) Option {
	return func(q *Queue) { q.retry = p.withDefaults() }
}

// WithWorkers sets background worker count for Run.
func WithWorkers(n int) Option {
	return func(q *Queue) {
		if n > 0 {
			q.workers = n
		}
	}
}

func (q *Queue) Name() string { return q.name }

func (q *Queue) Backend() qdriver.Backend { return q.backend }

// Push enqueues payload on queue (empty queue uses the default name).
func (q *Queue) Push(ctx context.Context, queue string, payload []byte) (qdriver.Job, error) {
	if queue == "" {
		queue = q.defaultQueue
	}
	return q.backend.Push(ctx, queue, payload)
}

// Dispatch pops one job, invokes handler, and emits lifecycle events.
// When handler is nil the Queue's default handler is used.
func (q *Queue) Dispatch(ctx context.Context, queue string, handler qdriver.Handler) error {
	if queue == "" {
		queue = q.defaultQueue
	}
	if handler == nil {
		handler = q.handler
	}
	if handler == nil {
		return fmt.Errorf("queue: Dispatch requires a handler")
	}

	job, err := q.backend.Pop(ctx, queue)
	if err != nil {
		return err
	}
	if job == nil {
		return nil
	}

	q.emit(ctx, EventProcessing, job, nil)
	if err := handler(ctx, job); err != nil {
		policy := q.retry.withDefaults()
		nextAttempt := job.Attempts() + 1
		if nextAttempt >= policy.MaxAttempts {
			q.emit(ctx, EventDead, job, err)
			dlq := dlqName(queue, policy.DLQSuffix)
			if _, pushErr := q.backend.Push(ctx, dlq, job.Payload()); pushErr != nil {
				return pushErr
			}
			_ = q.backend.Delete(ctx, job)
			return err
		}
		q.emit(ctx, EventFailed, job, err)
		delay := computeBackoff(policy, nextAttempt)
		_ = q.backend.Release(ctx, job, delay)
		return err
	}
	if err := q.backend.Delete(ctx, job); err != nil {
		return err
	}
	q.emit(ctx, EventProcessed, job, nil)
	return nil
}

func (q *Queue) emit(ctx context.Context, name string, job qdriver.Job, jobErr error) {
	if q.bus == nil {
		return
	}
	meta := map[string]string{
		"queue":    job.Queue(),
		"job_id":   job.ID(),
		"attempts": fmt.Sprintf("%d", job.Attempts()),
	}
	if jobErr != nil {
		meta["error"] = jobErr.Error()
	}
	_ = q.bus.Dispatch(ctx, events.Event{
		Name:     name,
		Payload:  job.Payload(),
		Metadata: meta,
	})
}

// Run starts worker goroutines that continuously Dispatch until ctx is
// canceled or Stop is called.
func (q *Queue) Run(ctx context.Context, queue string) error {
	if queue == "" {
		queue = q.defaultQueue
	}
	q.mu.Lock()
	if q.running {
		q.mu.Unlock()
		return fmt.Errorf("queue: Run already started on %q", q.name)
	}
	q.running = true
	q.stop = make(chan struct{})
	workers := q.workers
	q.mu.Unlock()

	stop := q.stop
	for i := 0; i < workers; i++ {
		q.wg.Add(1)
		go func() {
			defer q.wg.Done()
			backoff := runErrBackoffBase
			for {
				select {
				case <-ctx.Done():
					return
				case <-stop:
					return
				default:
				}
				popCtx, cancel := context.WithTimeout(ctx, runPopTimeout)
				err := q.Dispatch(popCtx, queue, nil)
				popErr := popCtx.Err()
				cancel()

				// A real backend error is one Dispatch surfaced that was
				// NOT caused by the per-pop timeout (an idle empty queue
				// is already paced by runPopTimeout) and NOT by parent ctx
				// cancellation (handled by the loop guard). Back off only
				// for genuine backend failures to avoid busy-spinning.
				if err != nil && popErr == nil && ctx.Err() == nil {
					if !q.sleep(ctx, stop, backoff) {
						return
					}
					if backoff < runErrBackoffMax {
						backoff *= 2
						if backoff > runErrBackoffMax {
							backoff = runErrBackoffMax
						}
					}
					continue
				}
				// Success or empty queue: stay responsive, reset backoff.
				backoff = runErrBackoffBase
			}
		}()
	}
	return nil
}

// sleep waits for d or until ctx is canceled or stop is closed. It returns
// false if the worker should exit (ctx canceled or stop closed), keeping
// Stop() prompt even mid-backoff.
func (q *Queue) sleep(ctx context.Context, stop <-chan struct{}, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-stop:
		return false
	case <-t.C:
		return true
	}
}

// Stop signals Run workers to exit and waits for them.
func (q *Queue) Stop() {
	q.mu.Lock()
	if !q.running {
		q.mu.Unlock()
		return
	}
	close(q.stop)
	q.mu.Unlock()
	q.wg.Wait()
	q.mu.Lock()
	q.running = false
	q.mu.Unlock()
}

// Shutdown releases backend resources.
func (q *Queue) Shutdown(ctx context.Context) error {
	q.Stop()
	return q.backend.Shutdown(ctx)
}
