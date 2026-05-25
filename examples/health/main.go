// Run with `go run ./examples/health` from the repo root.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/godx-jp/godx-platform-framework/framework"
	"github.com/godx-jp/godx-platform-framework/health"
)

func main() {
	ctx := context.Background()
	reg := health.NewRegistry()
	reg.RegisterProbe("clock", func(ctx context.Context) error {
		if time.Now().IsZero() {
			return context.Canceled
		}
		return nil
	})

	app := framework.New("health-example", "0.0.0").Use(health.ModuleWithRegistry(reg))
	if err := app.Init(ctx); err != nil {
		log.Fatalf("init: %v", err)
	}
	defer func() { _ = app.Shutdown(ctx) }()

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	log.Printf("health listening on %s (%s, %s)", addr, health.PathHealthz, health.PathReadyz)
	log.Fatal(http.ListenAndServe(addr, health.Handler(reg, health.Options{})))
}
