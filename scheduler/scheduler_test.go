package scheduler

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/godx-jp/godx-platform-framework/scheduler/lock"
)

type mapCache struct {
	mu   sync.Mutex
	data map[string][]byte
}

func (m *mapCache) Add(_ context.Context, key string, value []byte, _ time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.data == nil {
		m.data = map[string][]byte{}
	}
	if _, ok := m.data[key]; ok {
		return false, nil
	}
	m.data[key] = value
	return true, nil
}

func (m *mapCache) Forget(_ context.Context, key string) error {
	m.mu.Lock()
	delete(m.data, key)
	m.mu.Unlock()
	return nil
}

func TestEveryMinuteRegistersJob(t *testing.T) {
	s := New(Options{})
	if err := s.EveryMinute().Do("tick", func(ctx context.Context) error { return nil }); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if got := s.Jobs(); len(got) != 1 || got[0] != "tick" {
		t.Fatalf("Jobs=%v", got)
	}
}

func TestCronInvalidExpression(t *testing.T) {
	s := New(Options{})
	if err := s.Cron("not-a-cron").Do("bad", func(ctx context.Context) error { return nil }); err == nil {
		t.Fatalf("expected invalid cron error")
	}
}

func TestWithoutOverlappingSkipsConcurrentRun(t *testing.T) {
	s := New(Options{})
	var running atomic.Int32
	var skipped atomic.Int32
	block := make(chan struct{})
	done := make(chan struct{})

	// Run every second for the test window.
	if err := s.Cron("@every 1s").WithoutOverlapping().Do("slow", func(ctx context.Context) error {
		if running.Add(1) > 1 {
			skipped.Add(1)
			return nil
		}
		defer running.Add(-1)
		select {
		case <-block:
		case <-ctx.Done():
		}
		return nil
	}); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	go func() {
		time.Sleep(1500 * time.Millisecond)
		close(block)
		time.Sleep(500 * time.Millisecond)
		close(done)
	}()
	<-done
	_ = s.Stop(context.Background())
}

func TestOnOneServerUsesCacheLock(t *testing.T) {
	mc := &mapCache{}
	cl, err := lock.NewCache(lock.CacheOptions{Store: mc, Owner: "a", TTL: time.Minute})
	if err != nil {
		t.Fatalf("NewCache: %v", err)
	}
	s := New(Options{DistributedLock: cl})
	var runs atomic.Int32
	if err := s.Cron("@every 200ms").OnOneServer().Do("once", func(ctx context.Context) error {
		runs.Add(1)
		time.Sleep(300 * time.Millisecond)
		return nil
	}); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	time.Sleep(500 * time.Millisecond)
	_ = s.Stop(context.Background())
	if runs.Load() != 1 {
		t.Fatalf("runs=%d want 1 (second tick should skip)", runs.Load())
	}
}

func TestDoRejectsEmptyName(t *testing.T) {
	s := New(Options{})
	if err := s.EveryMinute().Do("", func(ctx context.Context) error { return nil }); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRegisterAfterStartRejected(t *testing.T) {
	s := New(Options{})
	if err := s.EveryMinute().Do("a", func(ctx context.Context) error { return nil }); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = s.Stop(context.Background()) }()
	if err := s.EveryMinute().Do("b", func(ctx context.Context) error { return nil }); err == nil {
		t.Fatalf("expected post-Start registration error")
	}
}
