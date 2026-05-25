package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/godx-jp/godx-platform-framework/events"
	"github.com/godx-jp/godx-platform-framework/scheduler/lock"
	"github.com/robfig/cron/v3"
)

// JobFunc is the callback invoked for each scheduled run.
type JobFunc func(ctx context.Context) error

// Options configures a Scheduler.
type Options struct {
	Location *time.Location
	DistributedLock lock.Mutex
	Bus             events.Bus
	DefaultTimeout  time.Duration
}

type Scheduler struct {
	cron    *cron.Cron
	parser  cron.Parser
	mu      sync.Mutex
	started bool
	jobs    []jobDef
	bus     events.Bus
	defaultTimeout time.Duration

	memLock  lock.Mutex
	distLock lock.Mutex
	runWG    sync.WaitGroup
}

type jobDef struct {
	name             string
	withoutOverlap   bool
	onOneServer      bool
	timeout          time.Duration
	fn               JobFunc
}

// New returns a Scheduler. DistributedLock may be set via Options or
// WithDistributedLock before Start.
func New(opts Options) *Scheduler {
	loc := time.UTC
	if opts.Location != nil {
		loc = opts.Location
	}
	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	s := &Scheduler{
		cron:           cron.New(cron.WithLocation(loc), cron.WithParser(parser)),
		parser:         parser,
		memLock:        lock.NewMemory(),
		distLock:       opts.DistributedLock,
		bus:            opts.Bus,
		defaultTimeout: opts.DefaultTimeout,
	}
	return s
}

// WithDistributedLock sets the cache-backed lock used by OnOneServer.
func (s *Scheduler) WithDistributedLock(m lock.Mutex) {
	s.mu.Lock()
	s.distLock = m
	s.mu.Unlock()
}

// EveryMinute schedules fn at the start of every minute.
func (s *Scheduler) EveryMinute() *Schedule {
	return s.Cron("@every 1m")
}

// Cron schedules fn according to a standard five-field cron expression.
func (s *Scheduler) Cron(expr string) *Schedule {
	return &Schedule{sched: s, expr: expr}
}

// Schedule is a fluent builder for one cron entry.
type Schedule struct {
	sched            *Scheduler
	expr             string
	withoutOverlap   bool
	onOneServer      bool
	timeout          time.Duration
}

// WithoutOverlapping skips a run when the previous invocation is still active.
func (sc *Schedule) WithoutOverlapping() *Schedule {
	sc.withoutOverlap = true
	return sc
}

// OnOneServer skips a run when another replica holds the distributed lock.
func (sc *Schedule) OnOneServer() *Schedule {
	sc.onOneServer = true
	return sc
}

// Timeout sets a per-run context deadline.
func (sc *Schedule) Timeout(d time.Duration) *Schedule {
	sc.timeout = d
	return sc
}

// Do registers name and fn with the parent Scheduler.
func (sc *Schedule) Do(name string, fn JobFunc) error {
	if name == "" {
		return fmt.Errorf("scheduler: job name is required")
	}
	if fn == nil {
		return fmt.Errorf("scheduler: job %q: nil callback", name)
	}
	def := jobDef{
		name:           name,
		withoutOverlap: sc.withoutOverlap,
		onOneServer:    sc.onOneServer,
		timeout:        sc.timeout,
		fn:             fn,
	}
	wrapped := func() {
		sc.sched.runJob(def)
	}
	s := sc.sched
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return fmt.Errorf("scheduler: cannot register %q after Start", name)
	}
	if _, err := s.parser.Parse(sc.expr); err != nil {
		return fmt.Errorf("scheduler: job %q: invalid cron %q: %w", name, sc.expr, err)
	}
	if _, err := s.cron.AddFunc(sc.expr, wrapped); err != nil {
		return fmt.Errorf("scheduler: job %q: %w", name, err)
	}
	s.jobs = append(s.jobs, def)
	return nil
}

func (s *Scheduler) runJob(def jobDef) {
	ctx := context.Background()
	if def.onOneServer {
		if s.distLock == nil {
			s.emit(ctx, EventSkipped, def.name, "lock_misconfigured", nil)
			return
		}
		release, ok, err := s.distLock.TryAcquire(ctx, def.name)
		if err != nil || !ok {
			s.emit(ctx, EventSkipped, def.name, "lock_busy", err)
			return
		}
		defer func() { _ = release() }()
	}

	if def.withoutOverlap {
		release, ok, err := s.memLock.TryAcquire(ctx, def.name)
		if err != nil || !ok {
			s.emit(ctx, EventSkipped, def.name, "overlap", err)
			return
		}
		defer func() { _ = release() }()
	}

	timeout := def.timeout
	if timeout <= 0 {
		timeout = s.defaultTimeout
	}
	runCtx := ctx
	var cancel context.CancelFunc
	if timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, timeout)
	}
	if cancel != nil {
		defer cancel()
	}

	s.runWG.Add(1)
	defer s.runWG.Done()
	s.emit(runCtx, EventStarted, def.name, "", nil)
	start := time.Now()
	err := def.fn(runCtx)
	if err != nil {
		s.emit(runCtx, EventFailed, def.name, err.Error(), err)
		return
	}
	s.emit(runCtx, EventFinished, def.name, fmt.Sprintf("%d", time.Since(start).Milliseconds()), nil)
}

func (s *Scheduler) emit(ctx context.Context, name, job, detail string, err error) {
	if s.bus == nil {
		return
	}
	meta := map[string]string{"job": job}
	if detail != "" {
		meta["detail"] = detail
	}
	if err != nil {
		meta["error"] = err.Error()
	}
	_ = s.bus.Dispatch(ctx, events.Event{Name: name, Metadata: meta})
}

// Start begins executing registered jobs. Safe to call once.
func (s *Scheduler) Start(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return nil
	}
	s.cron.Start()
	s.started = true
	return nil
}

// Stop halts the cron runner and waits for in-flight jobs.
func (s *Scheduler) Stop(ctx context.Context) error {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return nil
	}
	cronCtx := s.cron.Stop()
	s.started = false
	s.mu.Unlock()
	<-cronCtx.Done()
	done := make(chan struct{})
	go func() {
		s.runWG.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
	}
	return nil
}

// Jobs returns registered job names in registration order.
func (s *Scheduler) Jobs() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.jobs))
	for _, j := range s.jobs {
		out = append(out, j.name)
	}
	return out
}
