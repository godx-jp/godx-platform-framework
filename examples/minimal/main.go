// Minimal example: 25 lines, no HTTP, no infra — just SDK init + a log line
// with trace context.
//
// Run:
//
//	OBSERVABILITY_DRIVER=stdout go run .
package main

import (
	"context"
	"log"
	"time"

	"github.com/godx-jp/godx-platform-framework/framework"
	"github.com/godx-jp/godx-platform-framework/observability"
)

func main() {
	app := framework.New("minimal", "0.1.0").Use(observability.Module)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := app.Init(ctx); err != nil {
		log.Fatal(err)
	}
	defer func() {
		_ = app.Shutdown(context.Background())
	}()

	obs := observability.FromApp(app)

	ctx, span := obs.Tracer().Start(ctx, "say-hello")
	defer span.End()

	obs.Logger().InfoContext(ctx, "hello from godx-platform-framework",
		"driver", obs.Driver(),
		"hint", "trace_id is auto-injected",
	)
}
