package scheduler

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/godx-jp/godx-platform-framework/events"
	"github.com/godx-jp/godx-platform-framework/scheduler/lock"
)

// fakeLeasedLock is a test double that implements lock.LeasedMutex. Its
// TryAcquireLease hands back a lost channel the test can close on demand to
// simulate a distributed lease being lost (TTL expiry / theft) mid-run.
type fakeLeasedLock struct {
	lost     chan struct{}
	released chan struct{}
}

func newFakeLeasedLock() *fakeLeasedLock {
	return &fakeLeasedLock{
		lost:     make(chan struct{}),
		released: make(chan struct{}, 1),
	}
}

// TryAcquire keeps fakeLeasedLock a valid lock.Mutex; runJob prefers the
// leased path, so this delegates for completeness.
func (f *fakeLeasedLock) TryAcquire(ctx context.Context, key string) (func() error, bool, error) {
	release, _, ok, err := f.TryAcquireLease(ctx, key)
	return release, ok, err
}

func (f *fakeLeasedLock) TryAcquireLease(_ context.Context, _ string) (func() error, <-chan struct{}, bool, error) {
	return func() error {
		select {
		case f.released <- struct{}{}:
		default:
		}
		return nil
	}, f.lost, true, nil
}

var _ lock.LeasedMutex = (*fakeLeasedLock)(nil)

// TestLostLeaseCancelsRunningJob proves that when a leased lock's lost channel
// closes mid-run, runJob cancels the job context (the callback sees
// ctx.Done()) and records the run as skipped with detail "lock_lost" without a
// double-emit of failed/finished.
func TestLostLeaseCancelsRunningJob(t *testing.T) {
	fl := newFakeLeasedLock()
	bus := events.New()

	var (
		evMu     sync.Mutex
		recorded []string // "name|detail"
	)
	bus.Listen("schedule.*", func(_ context.Context, e events.Event) error {
		evMu.Lock()
		recorded = append(recorded, e.Name+"|"+e.Metadata["detail"])
		evMu.Unlock()
		return nil
	})

	s := New(Options{DistributedLock: fl, Bus: bus})

	started := make(chan struct{})
	var observedCancel bool
	done := make(chan struct{})

	go func() {
		defer close(done)
		s.runJob(jobDef{
			name:        "leased",
			onOneServer: true,
			fn: func(ctx context.Context) error {
				close(started)
				<-ctx.Done() // block until the lease loss cancels us
				observedCancel = true
				return ctx.Err()
			},
		})
	}()

	// Wait for the job to be running, then lose the lease.
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("job never started")
	}
	close(fl.lost)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("runJob did not return after lease lost — job context was not cancelled")
	}

	if !observedCancel {
		t.Fatal("job callback did not observe ctx.Done() after lease loss")
	}

	// Release must have been called (defer release()).
	select {
	case <-fl.released:
	default:
		t.Fatal("lock release was not called")
	}

	// Health must record the lock_lost skip, not a failure.
	h := s.Health()["leased"]
	if h.LastStatus != EventSkipped || h.LastDetail != "lock_lost" {
		t.Fatalf("health = %q/%q, want %q/%q", h.LastStatus, h.LastDetail, EventSkipped, "lock_lost")
	}

	// Events: exactly one started and one skipped|lock_lost, and NO
	// failed/finished double-emit for the same run.
	evMu.Lock()
	defer evMu.Unlock()
	var sawStarted, sawLost bool
	for _, e := range recorded {
		switch e {
		case EventStarted + "|":
			sawStarted = true
		case EventSkipped + "|lock_lost":
			sawLost = true
		}
		if strings.HasPrefix(e, EventFailed+"|") || strings.HasPrefix(e, EventFinished+"|") {
			t.Fatalf("unexpected double-emit event %q after lock_lost", e)
		}
	}
	if !sawStarted {
		t.Fatalf("missing %s event; got %v", EventStarted, recorded)
	}
	if !sawLost {
		t.Fatalf("missing %s|lock_lost event; got %v", EventSkipped, recorded)
	}
}
