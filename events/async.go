package events

import (
	"context"
	"errors"
	"sync"
)

// AsyncOptions tunes the async wrapper.
type AsyncOptions struct {
	// Workers is the number of background goroutines pulling from
	// the dispatch queue. Defaults to 1.
	Workers int

	// QueueSize is the buffered channel capacity. Defaults to 256.
	// When the queue is full, Dispatch blocks until a worker frees a
	// slot — adjust to your throughput.
	QueueSize int

	// OnError is invoked for every dispatch error returned by the
	// inner Bus. May be nil.
	OnError func(error)
}

// NewAsync wraps inner in an asynchronous dispatcher. Listeners still
// run synchronously inside a worker goroutine; Dispatch returns to
// the caller as soon as the job is queued. Close drains the queue
// before returning.
//
// Use NewAsync when fire-and-forget semantics are required —
// Laravel's `ShouldQueue` is the spiritual cousin. Callers no longer
// see listener errors directly; surface them via Options.OnError.
func NewAsync(inner Bus, opts AsyncOptions) Bus {
	if opts.Workers <= 0 {
		opts.Workers = 1
	}
	if opts.QueueSize <= 0 {
		opts.QueueSize = 256
	}
	a := &asyncBus{
		inner: inner,
		opts:  opts,
		queue: make(chan dispatchJob, opts.QueueSize),
		done:  make(chan struct{}),
	}
	a.wg.Add(opts.Workers)
	for i := 0; i < opts.Workers; i++ {
		go a.worker()
	}
	return a
}

type dispatchJob struct {
	ctx context.Context
	e   Event
}

type asyncBus struct {
	inner Bus
	opts  AsyncOptions

	mu     sync.Mutex
	closed bool

	queue chan dispatchJob
	done  chan struct{}
	wg    sync.WaitGroup
}

func (a *asyncBus) Listen(pattern string, l Listener) Subscription {
	return a.inner.Listen(pattern, l)
}

func (a *asyncBus) Forget(pattern string) int {
	return a.inner.Forget(pattern)
}

func (a *asyncBus) Patterns() []string {
	return a.inner.Patterns()
}

// Dispatch hands the Event off to a background worker and returns as
// soon as the queue accepts the job (or ctx is canceled, or the bus
// is closed).
func (a *asyncBus) Dispatch(ctx context.Context, e Event) error {
	a.mu.Lock()
	closed := a.closed
	a.mu.Unlock()
	if closed {
		return ErrClosed
	}
	select {
	case a.queue <- dispatchJob{ctx: ctx, e: e}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (a *asyncBus) worker() {
	defer a.wg.Done()
	for {
		select {
		case <-a.done:
			a.drain()
			return
		case job, ok := <-a.queue:
			if !ok {
				return
			}
			if err := a.inner.Dispatch(job.ctx, job.e); err != nil && a.opts.OnError != nil {
				a.opts.OnError(err)
			}
		}
	}
}

func (a *asyncBus) drain() {
	for {
		select {
		case job := <-a.queue:
			if err := a.inner.Dispatch(job.ctx, job.e); err != nil && a.opts.OnError != nil {
				a.opts.OnError(err)
			}
		default:
			return
		}
	}
}

// Close signals workers to drain and stop, waits for them, then
// closes the inner Bus. Returns ctx.Err if ctx expires before
// workers finish (the inner Bus is still closed in that case).
func (a *asyncBus) Close(ctx context.Context) error {
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return nil
	}
	a.closed = true
	close(a.done)
	a.mu.Unlock()

	finished := make(chan struct{})
	go func() {
		a.wg.Wait()
		close(finished)
	}()
	select {
	case <-finished:
	case <-ctx.Done():
		return errors.Join(ctx.Err(), a.inner.Close(ctx))
	}
	return a.inner.Close(ctx)
}
