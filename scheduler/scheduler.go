package scheduler

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/godx-jp/godx-platform-framework/events"
	"github.com/godx-jp/godx-platform-framework/scheduler/lock"
	"github.com/robfig/cron/v3"
)

// JobFunc is the callback invoked for each scheduled run.
type JobFunc func(ctx context.Context) error

// QueuePushFunc dispatches a payload to a named queue (RunOnQueue).
type QueuePushFunc func(ctx context.Context, queue string, payload []byte) error

// Options configures a Scheduler.
type Options struct {
	Location        *time.Location
	DistributedLock lock.Mutex
	Bus             events.Bus
	DefaultTimeout  time.Duration
	QueuePush       QueuePushFunc
	OnRun           func(job string, err error)
}

type Scheduler struct {
	cron           *cron.Cron
	parser         cron.Parser
	mu             sync.Mutex
	started        bool
	jobs           []jobDef
	bus            events.Bus
	defaultTimeout time.Duration
	queuePush      QueuePushFunc
	onRun          func(job string, err error)

	memLock  lock.Mutex
	distLock lock.Mutex
	runWG    sync.WaitGroup

	healthMu sync.RWMutex
	health   map[string]JobHealth
}

type jobDef struct {
	name           string
	withoutOverlap bool
	onOneServer    bool
	timeout        time.Duration
	environments   []string
	betweenStart   string
	betweenEnd     string
	when           func() bool
	unless         func() bool
	runOnQueue     string
	fn             JobFunc
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
		queuePush:      opts.QueuePush,
		onRun:          opts.OnRun,
		health:         map[string]JobHealth{},
	}
	return s
}

// WithDistributedLock sets the cache-backed lock used by OnOneServer.
func (s *Scheduler) WithDistributedLock(m lock.Mutex) {
	s.mu.Lock()
	s.distLock = m
	s.mu.Unlock()
}

// WithQueuePush sets the callback used by RunOnQueue schedules.
func (s *Scheduler) WithQueuePush(fn QueuePushFunc) {
	s.mu.Lock()
	s.queuePush = fn
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
	sched          *Scheduler
	expr           string
	withoutOverlap bool
	onOneServer    bool
	timeout        time.Duration
	environments   []string
	betweenStart   string
	betweenEnd     string
	when           func() bool
	unless         func() bool
	runOnQueue     string
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

// Environments limits execution to the given APP_ENV values.
func (sc *Schedule) Environments(envs ...string) *Schedule {
	sc.environments = append([]string(nil), envs...)
	return sc
}

// Between limits execution to a daily time window (HH:MM, 24h).
func (sc *Schedule) Between(start, end string) *Schedule {
	sc.betweenStart = start
	sc.betweenEnd = end
	return sc
}

// When skips the run when fn returns false.
func (sc *Schedule) When(fn func() bool) *Schedule {
	sc.when = fn
	return sc
}

// Unless skips the run when fn returns true.
func (sc *Schedule) Unless(fn func() bool) *Schedule {
	sc.unless = fn
	return sc
}

// RunOnQueue dispatches payload to queue instead of running fn inline.
func (sc *Schedule) RunOnQueue(queue string) *Schedule {
	sc.runOnQueue = queue
	return sc
}

// Do registers name and fn with the parent Scheduler.
func (sc *Schedule) Do(name string, fn JobFunc) error {
	if name == "" {
		return fmt.Errorf("scheduler: job name is required")
	}
	if fn == nil && sc.runOnQueue == "" {
		return fmt.Errorf("scheduler: job %q: nil callback", name)
	}
	def := jobDef{
		name:           name,
		withoutOverlap: sc.withoutOverlap,
		onOneServer:    sc.onOneServer,
		timeout:        sc.timeout,
		environments:   sc.environments,
		betweenStart:   sc.betweenStart,
		betweenEnd:     sc.betweenEnd,
		when:           sc.when,
		unless:         sc.unless,
		runOnQueue:     sc.runOnQueue,
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

	if MaintenanceMode() {
		s.emit(ctx, EventSkipped, def.name, "maintenance", nil)
		s.recordHealth(def.name, EventSkipped, "maintenance")
		return
	}
	if !envAllowed(def.environments) {
		s.emit(ctx, EventSkipped, def.name, "environment", nil)
		s.recordHealth(def.name, EventSkipped, "environment")
		return
	}
	if !withinBetween(time.Now(), def.betweenStart, def.betweenEnd) {
		s.emit(ctx, EventSkipped, def.name, "between", nil)
		s.recordHealth(def.name, EventSkipped, "between")
		return
	}
	if def.when != nil && !def.when() {
		s.emit(ctx, EventSkipped, def.name, "when", nil)
		s.recordHealth(def.name, EventSkipped, "when")
		return
	}
	if def.unless != nil && def.unless() {
		s.emit(ctx, EventSkipped, def.name, "unless", nil)
		s.recordHealth(def.name, EventSkipped, "unless")
		return
	}

	// lockLost is closed by a leased distributed lock when the lease is lost
	// mid-run (renewal failed or the key expired/was stolen). It stays nil for
	// memory locks and dist locks that do not implement lock.LeasedMutex.
	var lockLost <-chan struct{}
	if def.onOneServer {
		if s.distLock == nil {
			s.emit(ctx, EventSkipped, def.name, "lock_misconfigured", nil)
			s.recordHealth(def.name, EventSkipped, "lock_misconfigured")
			return
		}
		if leased, ok := s.distLock.(lock.LeasedMutex); ok {
			release, lost, acquired, err := leased.TryAcquireLease(ctx, def.name)
			if err != nil || !acquired {
				s.emit(ctx, EventSkipped, def.name, "lock_busy", err)
				s.recordHealth(def.name, EventSkipped, "lock_busy")
				return
			}
			lockLost = lost
			defer func() { _ = release() }()
		} else {
			release, ok, err := s.distLock.TryAcquire(ctx, def.name)
			if err != nil || !ok {
				s.emit(ctx, EventSkipped, def.name, "lock_busy", err)
				s.recordHealth(def.name, EventSkipped, "lock_busy")
				return
			}
			defer func() { _ = release() }()
		}
	}

	if def.withoutOverlap {
		release, ok, err := s.memLock.TryAcquire(ctx, def.name)
		if err != nil || !ok {
			s.emit(ctx, EventSkipped, def.name, "overlap", err)
			s.recordHealth(def.name, EventSkipped, "overlap")
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
	} else if lockLost != nil {
		// A leased lock can be lost mid-run even without a timeout, so we need
		// a cancellable context to abort the job when the lease drops.
		runCtx, cancel = context.WithCancel(ctx)
	}
	if cancel != nil {
		defer cancel()
	}

	// Watch the lease: if it is lost while the job runs, cancel runCtx so the
	// callback observes ctx.Done(), and record the run as skipped with reason
	// "lock_lost". The watcher always exits via one of the two select arms
	// (lease lost, or the run finished and cancel() fired through defer), so
	// there is no goroutine leak. lockLostFired guards against the lost-event
	// being emitted twice and lets the post-run path know cancellation came
	// from a lost lease rather than a normal timeout.
	var lockLostFired atomic.Bool
	if lockLost != nil {
		watchDone := make(chan struct{})
		defer func() {
			cancel()
			<-watchDone
		}()
		go func() {
			defer close(watchDone)
			select {
			case <-lockLost:
				lockLostFired.Store(true)
				cancel()
				s.emit(ctx, EventSkipped, def.name, "lock_lost", nil)
				s.recordHealth(def.name, EventSkipped, "lock_lost")
			case <-runCtx.Done():
			}
		}()
	}

	s.runWG.Add(1)
	defer s.runWG.Done()
	s.emit(runCtx, EventStarted, def.name, "", nil)
	start := time.Now()

	var err error
	if def.runOnQueue != "" {
		if s.queuePush == nil {
			err = fmt.Errorf("scheduler: QueuePush not configured")
		} else {
			err = s.queuePush(runCtx, def.runOnQueue, []byte(def.name))
		}
	} else {
		err = def.fn(runCtx)
	}

	if s.onRun != nil {
		s.onRun(def.name, err)
	}
	// If the lease was lost, the watcher already emitted EventSkipped/"lock_lost"
	// and recorded health; the callback's (likely context-cancelled) result is
	// reported to onRun above but must not also produce a failed/finished event,
	// which would be a misleading double-emit for the same run.
	if lockLostFired.Load() {
		return
	}
	if err != nil {
		s.emit(runCtx, EventFailed, def.name, err.Error(), err)
		s.recordHealth(def.name, EventFailed, err.Error())
		return
	}
	s.emit(runCtx, EventFinished, def.name, fmt.Sprintf("%d", time.Since(start).Milliseconds()), nil)
	s.recordHealth(def.name, EventFinished, fmt.Sprintf("%dms", time.Since(start).Milliseconds()))
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
