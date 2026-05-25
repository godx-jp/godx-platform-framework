package memory

import (
	"context"
	"testing"
	"time"

	qdriver "github.com/godx-jp/godx-platform-framework/queue/driver"
)

func TestPushPopRoundTrip(t *testing.T) {
	b, err := construct(context.Background(), qdriver.Spec{DefaultQueue: "jobs", QueueSize: 8})
	if err != nil {
		t.Fatal(err)
	}
	defer b.Shutdown(context.Background())

	if _, err := b.Push(context.Background(), "jobs", []byte("hello")); err != nil {
		t.Fatal(err)
	}
	job, err := b.Pop(context.Background(), "jobs")
	if err != nil {
		t.Fatal(err)
	}
	if string(job.Payload()) != "hello" {
		t.Fatalf("payload=%q", job.Payload())
	}
	if err := b.Delete(context.Background(), job); err != nil {
		t.Fatal(err)
	}
}

func TestReleaseIncrementsAttempts(t *testing.T) {
	b, _ := construct(context.Background(), qdriver.Spec{QueueSize: 4})
	ctx := context.Background()
	job, _ := b.Push(ctx, "", []byte("x"))
	_ = b.Release(ctx, job, 0)
	job2, err := b.Pop(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if job2.Attempts() != 1 {
		t.Fatalf("attempts=%d", job2.Attempts())
	}
}

func TestShutdownClosed(t *testing.T) {
	b, _ := construct(context.Background(), qdriver.Spec{})
	_ = b.Shutdown(context.Background())
	if _, err := b.Push(context.Background(), "", []byte("x")); err != qdriver.ErrClosed {
		t.Fatalf("Push err=%v", err)
	}
}

func TestPopRespectsContext(t *testing.T) {
	b, _ := construct(context.Background(), qdriver.Spec{})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := b.Pop(ctx, "")
	if err != context.DeadlineExceeded {
		t.Fatalf("err=%v", err)
	}
}

func TestRegisteredOnImport(t *testing.T) {
	if qdriver.Lookup(qdriver.DriverMemory) == nil {
		t.Fatal("memory driver not registered")
	}
}
