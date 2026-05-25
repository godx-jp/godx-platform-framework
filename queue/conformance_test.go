package queue_test

import (
	"context"
	"errors"
	"testing"
	"time"

	qdriver "github.com/godx-jp/godx-platform-framework/queue/driver"
	_ "github.com/godx-jp/godx-platform-framework/queue/drivers/memory"
)

type backendCase struct {
	name  string
	build func(t *testing.T) qdriver.Backend
}

func memoryCase() backendCase {
	return backendCase{
		name: qdriver.DriverMemory,
		build: func(t *testing.T) qdriver.Backend {
			t.Helper()
			b, err := qdriver.New(context.Background(), qdriver.Spec{
				Name:         qdriver.DriverMemory,
				DefaultQueue: "jobs",
				QueueSize:    16,
			})
			if err != nil {
				t.Fatalf("build: %v", err)
			}
			return b
		},
	}
}

func runConformance(t *testing.T, bc backendCase) {
	t.Run(bc.name, func(t *testing.T) {
		t.Run("name_matches_driver", func(t *testing.T) {
			b := bc.build(t)
			defer b.Shutdown(context.Background())
			if b.Name() != bc.name {
				t.Fatalf("Name=%q want %q", b.Name(), bc.name)
			}
		})

		t.Run("push_pop_delete", func(t *testing.T) {
			b := bc.build(t)
			defer b.Shutdown(context.Background())
			ctx := context.Background()
			if _, err := b.Push(ctx, "jobs", []byte("hello")); err != nil {
				t.Fatalf("Push: %v", err)
			}
			job, err := b.Pop(ctx, "jobs")
			if err != nil {
				t.Fatalf("Pop: %v", err)
			}
			if string(job.Payload()) != "hello" {
				t.Fatalf("payload=%q", job.Payload())
			}
			if err := b.Delete(ctx, job); err != nil {
				t.Fatalf("Delete: %v", err)
			}
		})

		t.Run("release_increments_attempts", func(t *testing.T) {
			b := bc.build(t)
			defer b.Shutdown(context.Background())
			ctx := context.Background()
			job, err := b.Push(ctx, "", []byte("x"))
			if err != nil {
				t.Fatalf("Push: %v", err)
			}
			if err := b.Release(ctx, job, 0); err != nil {
				t.Fatalf("Release: %v", err)
			}
			job2, err := b.Pop(ctx, "")
			if err != nil {
				t.Fatalf("Pop: %v", err)
			}
			if job2.Attempts() != 1 {
				t.Fatalf("attempts=%d", job2.Attempts())
			}
		})

		t.Run("pop_respects_context", func(t *testing.T) {
			b := bc.build(t)
			defer b.Shutdown(context.Background())
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()
			_, err := b.Pop(ctx, "jobs")
			if err != context.DeadlineExceeded {
				t.Fatalf("err=%v", err)
			}
		})

		t.Run("shutdown_is_idempotent_and_blocks_ops", func(t *testing.T) {
			b := bc.build(t)
			if err := b.Shutdown(context.Background()); err != nil {
				t.Fatalf("first: %v", err)
			}
			if err := b.Shutdown(context.Background()); err != nil {
				t.Fatalf("second: %v", err)
			}
			_, err := b.Push(context.Background(), "", []byte("x"))
			if !errors.Is(err, qdriver.ErrClosed) {
				t.Fatalf("Push after Shutdown err=%v", err)
			}
		})
	})
}

func TestConformance(t *testing.T) {
	runConformance(t, memoryCase())
}
