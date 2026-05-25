package scheduler

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/godx-jp/godx-platform-framework/scheduler/lock"
)

func TestRunJobDirectOnOneServer(t *testing.T) {
	mc := &mapCache{}
	cl, err := lock.NewCache(lock.CacheOptions{Store: mc, Owner: "a"})
	if err != nil {
		t.Fatal(err)
	}
	s := New(Options{DistributedLock: cl})
	var runs atomic.Int32
	s.runJob(jobDef{
		name:        "once",
		onOneServer: true,
		fn: func(context.Context) error {
			runs.Add(1)
			return nil
		},
	})
	if runs.Load() != 1 {
		t.Fatalf("runs=%d", runs.Load())
	}
}
