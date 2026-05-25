// Run with `go run ./examples/config` from the repo root.
//
// Zero-config (process env only, no prefix):
//
//	APP_NAME=tiximax APP_PORT=8080 go run ./examples/config
//
// With a file source:
//
//	cat > /tmp/cfg.yaml <<'YAML'
//	app:
//	  name: tiximax
//	  port: 9090
//	  features:
//	    - alpha
//	    - beta
//	YAML
//	CONFIG_SOURCES=file \
//	CONFIG_SOURCE_FILE_PATH=/tmp/cfg.yaml \
//	go run ./examples/config
//
// With layered file + env override (env wins because AutoEnv runs last):
//
//	APP_PORT=7777 \
//	CONFIG_SOURCES=file \
//	CONFIG_SOURCE_FILE_PATH=/tmp/cfg.yaml \
//	CONFIG_ENV_PREFIX=APP_ \
//	go run ./examples/config
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/godx-jp/godx-platform-framework/config"
	"github.com/godx-jp/godx-platform-framework/framework"
)

func main() {
	ctx := context.Background()
	app := framework.New("config-example", "0.0.0").Use(config.Module)
	if err := app.Init(ctx); err != nil {
		log.Fatalf("init: %v", err)
	}
	defer func() { _ = app.Shutdown(ctx) }()

	cfg, err := config.FromApp(app)
	if err != nil {
		log.Fatalf("config not wired: %v", err)
	}

	name := cfg.GetString("app.name", "unset")
	port := cfg.GetInt("app.port", 8080)
	ttl := cfg.GetDuration("app.ttl", 5*time.Second)
	flags := cfg.GetStringSlice("app.features", nil)

	fmt.Printf("app.name      = %q\n", name)
	fmt.Printf("app.port      = %d\n", port)
	fmt.Printf("app.ttl       = %s\n", ttl)
	fmt.Printf("app.features  = %v\n", flags)
	fmt.Printf("flat keys     = %d\n", len(cfg.AllFlat()))

	// Generic typed accessor — handy in handler code.
	timeout := config.Get[time.Duration](cfg, "http.client.timeout", 2*time.Second)
	fmt.Printf("http timeout  = %s\n", timeout)
}
