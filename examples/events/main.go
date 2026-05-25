// Run with `go run ./examples/events` from the repo root.
//
// Synchronous dispatch (the default):
//
//	go run ./examples/events
//
// Async dispatch with 4 workers:
//
//	EVENTS_ASYNC=true EVENTS_ASYNC_WORKERS=4 go run ./examples/events
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/godx-jp/godx-platform-framework/events"
	"github.com/godx-jp/godx-platform-framework/framework"
)

func main() {
	ctx := context.Background()
	app := framework.New("events-example", "0.0.0").Use(events.Module)
	if err := app.Init(ctx); err != nil {
		log.Fatalf("init: %v", err)
	}
	defer func() { _ = app.Shutdown(ctx) }()

	bus, err := events.FromApp(app)
	if err != nil {
		log.Fatalf("events not wired: %v", err)
	}

	// Exact match.
	bus.Listen("user.created", func(ctx context.Context, e events.Event) error {
		fmt.Printf("[welcome-mailer] user %v\n", e.Payload)
		return nil
	})
	// Wildcard fan-out for audit logging.
	bus.Listen("user.*", func(ctx context.Context, e events.Event) error {
		fmt.Printf("[audit] %s at %s with %v\n", e.Name, e.CreatedAt.Format(time.RFC3339Nano), e.Payload)
		return nil
	})
	// Catch-all for metrics.
	bus.Listen("*", func(ctx context.Context, e events.Event) error {
		fmt.Printf("[metric] event=%s\n", e.Name)
		return nil
	})

	if err := bus.Dispatch(ctx, events.Event{Name: "user.created", Payload: "alice@example.com"}); err != nil {
		log.Printf("dispatch err: %v", err)
	}
	if err := bus.Dispatch(ctx, events.Event{Name: "user.deleted", Payload: "bob@example.com"}); err != nil {
		log.Printf("dispatch err: %v", err)
	}
	if err := bus.Dispatch(ctx, events.Event{Name: "order.placed", Payload: 1234}); err != nil {
		log.Printf("dispatch err: %v", err)
	}

	// In async mode, give workers a moment before Shutdown.
	time.Sleep(100 * time.Millisecond)
}
