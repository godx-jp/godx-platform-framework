// Run with `go run ./examples/scheduler` from the repo root.
//
// Disable auto-start to register jobs before Start:
//
//	SCHEDULER_ENABLED=false go run ./examples/scheduler
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/godx-jp/godx-platform-framework/framework"
	"github.com/godx-jp/godx-platform-framework/scheduler"
)

func main() {
	ctx := context.Background()
	cfg := scheduler.Config{Enabled: false}
	app := framework.New("scheduler-example", "0.0.0").Use(scheduler.ModuleWithConfig(cfg, nil))
	if err := app.Init(ctx); err != nil {
		log.Fatalf("init: %v", err)
	}
	defer func() { _ = app.Shutdown(ctx) }()

	sched, err := scheduler.FromApp(app)
	if err != nil {
		log.Fatalf("scheduler not wired: %v", err)
	}

	if err := sched.EveryMinute().WithoutOverlapping().Do("heartbeat", func(ctx context.Context) error {
		fmt.Println("[heartbeat]", time.Now().Format(time.RFC3339))
		return nil
	}); err != nil {
		log.Fatalf("register heartbeat: %v", err)
	}
	if err := sched.Cron("@every 5s").Do("demo", func(ctx context.Context) error {
		fmt.Println("[demo]", time.Now().Format(time.RFC3339))
		return nil
	}); err != nil {
		log.Fatalf("register demo: %v", err)
	}

	if err := sched.Start(ctx); err != nil {
		log.Fatalf("start: %v", err)
	}
	fmt.Println("scheduler running — Ctrl+C to stop")
	time.Sleep(12 * time.Second)
}
