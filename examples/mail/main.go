// Run with `go run ./examples/mail` from the repo root.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/godx-jp/godx-platform-framework/events"
	"github.com/godx-jp/godx-platform-framework/framework"
	"github.com/godx-jp/godx-platform-framework/mail"
)

func main() {
	ctx := context.Background()
	app := framework.New("mail-example", "0.0.0").
		Use(events.Module).
		Use(mail.Module)
	if err := app.Init(ctx); err != nil {
		log.Fatalf("init: %v", err)
	}
	defer func() { _ = app.Shutdown(ctx) }()

	bus, _ := events.FromApp(app)
	bus.Listen("mail.*", func(_ context.Context, e events.Event) error {
		fmt.Printf("event %s: %v\n", e.Name, e.Payload)
		return nil
	})

	mgr, err := mail.FromApp(app)
	if err != nil {
		log.Fatalf("mail: %v", err)
	}
	ml, err := mgr.Mailer()
	if err != nil {
		log.Fatalf("Mailer: %v", err)
	}
	if err := ml.To("user@example.com").Subject("Welcome").Body("Hello from godx mail.").Send(ctx); err != nil {
		log.Fatalf("send: %v", err)
	}
	fmt.Println("mail sent (check slog output for log driver)")
}
