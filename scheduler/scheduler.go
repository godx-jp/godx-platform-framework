package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/godx-jp/godx-platform-framework/scheduler/lock"
	"github.com/robfig/cron/v3"
)

// JobFunc is the callback invoked for each scheduled run.
type JobFunc func(ctx context.Context) error

// Options configures a Scheduler.
type Options struct {
	// Location sets the cron timezone. Nil uses UTC.
	Location *time.Location
	// DistributedLock enables OnOneServer when non-nil.
	DistributedLock lock.Mutex
}

// Scheduler registers and runs cron jobs.
type Scheduler struct {
	cron    *cron.Cron
	parser  cron.Parser
	mu      sync.Mutex
	started bool
	jobs    []jobDef

	memLock  lock.Mutex
	distLock lock.Mutex
}

type jobDef struct {
	name             string
	withoutOverlap   bool
	onOneServer      bool
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
		cron:     cron.New(cron.WithLocation(loc), cron.WithParser(parser)),
		parser:   parser,
		memLock:  lock.NewMemory(),
		distLock: opts.DistributedLock,
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
			return
		}
		release, ok, err := s.distLock.TryAcquire(ctx, def.name)
		if err != nil || !ok {
			return
		}
		defer func() { _ = release() }()
	}

	if def.withoutOverlap {
		release, ok, err := s.memLock.TryAcquire(ctx, def.name)
		if err != nil || !ok {
			return
		}
		defer func() { _ = release() }()
	}

	_ = def.fn(ctx)
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
func (s *Scheduler) Stop(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.started {
		return nil
	}
	ctx := s.cron.Stop()
	<-ctx.Done()
	s.started = false
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
