// Run with `go run ./examples/queue` from the repo root.
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/godx-jp/godx-platform-framework/events"
	"github.com/godx-jp/godx-platform-framework/framework"
	"github.com/godx-jp/godx-platform-framework/queue"
	qdriver "github.com/godx-jp/godx-platform-framework/queue/driver"
)

func main() {
	ctx := context.Background()
	bus := events.New()
	bus.Listen(queue.EventProcessing, audit("processing"))
	bus.Listen(queue.EventProcessed, audit("processed"))
	bus.Listen(queue.EventFailed, audit("failed"))

	app := framework.New("queue-example", "0.0.0").Use(
		queue.ModuleWithConfig(queue.Config{
			Default: "default",
			Queues: map[string]queue.QueueConfig{
				"default": {Driver: "memory", DefaultQueue: "emails", Workers: 2, QueueSize: 32},
			},
		}, bus),
	)
	if err := app.Init(ctx); err != nil {
		log.Fatalf("init: %v", err)
	}
	defer func() { _ = app.Shutdown(ctx) }()

	mgr, _ := queue.FromApp(app)
	q, _ := mgr.Default()
	q = queue.NewQueue(q.Name(), q.Backend(),
		queue.WithBus(bus),
		queue.WithHandler(sendEmail),
		queue.WithWorkers(2),
	)

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	_ = q.Run(runCtx, "emails")

	for _, addr := range []string{"alice@example.com", "bob@example.com"} {
		if _, err := q.Push(ctx, "emails", []byte(addr)); err != nil {
			log.Printf("push: %v", err)
		}
	}
	time.Sleep(100 * time.Millisecond)
	q.Stop()
}

func sendEmail(ctx context.Context, job qdriver.Job) error {
	fmt.Printf("[mailer] sending welcome to %s\n", job.Payload())
	return nil
}

func audit(label string) events.Listener {
	return func(ctx context.Context, e events.Event) error {
		fmt.Printf("[audit] %s job_id=%s queue=%s\n", label, e.Metadata["job_id"], e.Metadata["queue"])
		return nil
	}
}
