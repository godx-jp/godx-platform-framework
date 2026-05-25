// Package memory provides an in-process queue backed by buffered channels.
// Light driver — auto-registers under "memory".
package memory

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	qdriver "github.com/godx-jp/godx-platform-framework/queue/driver"
)

func init() {
	qdriver.Register(qdriver.DriverMemory, construct)
}

const Name = qdriver.DriverMemory

func construct(_ context.Context, spec qdriver.Spec) (qdriver.Backend, error) {
	size := spec.QueueSize
	if size <= 0 {
		size = 256
	}
	d := &impl{
		defaultQueue: spec.DefaultQueue,
		channels:     make(map[string]chan *memJob),
		capacity:     size,
	}
	if d.defaultQueue == "" {
		d.defaultQueue = "default"
	}
	return d, nil
}

type memJob struct {
	id       string
	queue    string
	payload  []byte
	attempts int
}

func (j *memJob) ID() string       { return j.id }
func (j *memJob) Queue() string    { return j.queue }
func (j *memJob) Payload() []byte  { return append([]byte(nil), j.payload...) }
func (j *memJob) Attempts() int    { return j.attempts }

type impl struct {
	defaultQueue string
	capacity     int

	mu       sync.Mutex
	closed   bool
	nextID   atomic.Uint64
	channels map[string]chan *memJob
	delayed  []*delayedJob
}

type delayedJob struct {
	job   *memJob
	runAt time.Time
}

func (d *impl) Name() string { return qdriver.DriverMemory }

func (d *impl) channel(queue string) chan *memJob {
	if queue == "" {
		queue = d.defaultQueue
	}
	ch, ok := d.channels[queue]
	if !ok {
		ch = make(chan *memJob, d.capacity)
		d.channels[queue] = ch
	}
	return ch
}

func (d *impl) Push(_ context.Context, queue string, payload []byte) (qdriver.Job, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return nil, qdriver.ErrClosed
	}
	if queue == "" {
		queue = d.defaultQueue
	}
	job := &memJob{
		id:       fmt.Sprintf("%d", d.nextID.Add(1)),
		queue:    queue,
		payload:  append([]byte(nil), payload...),
		attempts: 0,
	}
	select {
	case d.channel(queue) <- job:
		return job, nil
	default:
		return nil, fmt.Errorf("queue/memory: queue %q full (capacity %d)", queue, d.capacity)
	}
}

func (d *impl) Pop(ctx context.Context, queue string) (qdriver.Job, error) {
	if queue == "" {
		queue = d.defaultQueue
	}
	for {
		d.mu.Lock()
		if d.closed {
			d.mu.Unlock()
			return nil, qdriver.ErrClosed
		}
		now := time.Now()
		var ready *memJob
		remaining := d.delayed[:0]
		for _, dj := range d.delayed {
			if dj.job.queue != queue {
				remaining = append(remaining, dj)
				continue
			}
			if !now.Before(dj.runAt) {
				ready = dj.job
				break
			}
			remaining = append(remaining, dj)
		}
		d.delayed = remaining
		d.mu.Unlock()

		if ready != nil {
			return ready, nil
		}

		d.mu.Lock()
		ch := d.channel(queue)
		d.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case job, ok := <-ch:
			if !ok {
				return nil, qdriver.ErrClosed
			}
			return job, nil
		}
	}
}

func (d *impl) Delete(_ context.Context, _ qdriver.Job) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return qdriver.ErrClosed
	}
	return nil
}

func (d *impl) Release(_ context.Context, job qdriver.Job, delay time.Duration) error {
	mj, ok := job.(*memJob)
	if !ok {
		return fmt.Errorf("queue/memory: unknown job type %T", job)
	}
	mj.attempts++
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return qdriver.ErrClosed
	}
	if delay <= 0 {
		select {
		case d.channel(mj.queue) <- mj:
			return nil
		default:
			return fmt.Errorf("queue/memory: queue %q full", mj.queue)
		}
	}
	d.delayed = append(d.delayed, &delayedJob{job: mj, runAt: time.Now().Add(delay)})
	return nil
}

func (d *impl) Shutdown(_ context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return nil
	}
	d.closed = true
	for _, ch := range d.channels {
		close(ch)
	}
	return nil
}
