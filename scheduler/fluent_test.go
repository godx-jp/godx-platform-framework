package scheduler

import (
	"context"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/godx-jp/godx-platform-framework/scheduler/lock"
)

func TestWeeklyOnRegistersCron(t *testing.T) {
	s := New(Options{})
	if err := s.WeeklyOn(time.Monday, "09:30").Do("weekly", func(ctx context.Context) error { return nil }); err != nil {
		t.Fatalf("Do: %v", err)
	}
}

func TestEnvironmentsSkipsWrongEnv(t *testing.T) {
	t.Setenv("APP_ENV", "staging")
	s := New(Options{})
	var runs atomic.Int32
	s.runJob(jobDef{
		name:         "env-job",
		environments: []string{"production"},
		fn:           func(context.Context) error { runs.Add(1); return nil },
	})
	if runs.Load() != 0 {
		t.Fatalf("runs=%d want 0", runs.Load())
	}
}

func TestWhenUnless(t *testing.T) {
	s := New(Options{})
	var runs atomic.Int32
	s.runJob(jobDef{
		name: "w",
		when: func() bool { return false },
		fn:   func(context.Context) error { runs.Add(1); return nil },
	})
	s.runJob(jobDef{
		name:   "u",
		unless: func() bool { return true },
		fn:     func(context.Context) error { runs.Add(1); return nil },
	})
	if runs.Load() != 0 {
		t.Fatalf("runs=%d", runs.Load())
	}
}

func TestRunOnQueue(t *testing.T) {
	var pushed atomic.Int32
	s := New(Options{
		QueuePush: func(_ context.Context, queue string, payload []byte) error {
			if queue != "jobs" || string(payload) != "dispatch-me" {
				t.Fatalf("queue=%q payload=%q", queue, payload)
			}
			pushed.Add(1)
			return nil
		},
	})
	s.runJob(jobDef{
		name:       "dispatch-me",
		runOnQueue: "jobs",
	})
	if pushed.Load() != 1 {
		t.Fatalf("pushed=%d", pushed.Load())
	}
}

func TestMaintenanceModeSkips(t *testing.T) {
	SetMaintenanceMode(true)
	defer SetMaintenanceMode(false)
	s := New(Options{})
	var runs atomic.Int32
	s.runJob(jobDef{
		name: "m",
		fn:   func(context.Context) error { runs.Add(1); return nil },
	})
	if runs.Load() != 0 {
		t.Fatalf("runs=%d", runs.Load())
	}
}

func TestHealthRecordsLastRun(t *testing.T) {
	s := New(Options{})
	s.runJob(jobDef{
		name: "h",
		fn:   func(context.Context) error { return nil },
	})
	if _, ok := s.LastRun("h"); !ok {
		t.Fatal("expected LastRun")
	}
	h := s.Health()["h"]
	if h.LastStatus != EventFinished {
		t.Fatalf("status=%q", h.LastStatus)
	}
}

func TestMapCacheRenew(t *testing.T) {
	mc := &mapCache{}
	if err := mc.Renew(context.Background(), "k", []byte("a"), time.Minute); err == nil {
		t.Fatal("expected renew error on missing key")
	}
	added, err := mc.Add(context.Background(), "k", []byte("a"), time.Minute)
	if err != nil || !added {
		t.Fatalf("Add: added=%v err=%v", added, err)
	}
	if err := mc.Renew(context.Background(), "k", []byte("a"), time.Minute); err != nil {
		t.Fatalf("Renew: %v", err)
	}
}

func TestRedisLockRequiresClient(t *testing.T) {
	if _, err := lock.NewRedis(lock.RedisOptions{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestCurrentEnvironmentDefault(t *testing.T) {
	os.Unsetenv("APP_ENV")
	if got := CurrentEnvironment(); got != "production" {
		t.Fatalf("got %q", got)
	}
}
