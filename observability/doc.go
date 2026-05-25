// Package observability is the framework module that gives services a
// single, opinionated handle for logs, traces, and metrics. It is the first
// concern shipped under godx-platform-framework; future modules (storage,
// cache, queue, ...) follow the same layout:
//
//   - Top-level package holds the public API (Config, Provider, Module).
//   - driver/ defines the contract third-party adapters must implement.
//   - drivers/<name>/ holds each built-in implementation in its own package
//     so callers only pay for what they import.
//   - middleware/ wraps the optional HTTP integration so net/http stays out
//     of the core dependency graph for non-HTTP callers.
//
// Light drivers (stdout, file, stack) are auto-registered when this package
// is imported. Heavy drivers (otlp, cloudwatch) require an explicit blank
// import — see drivers/otlp/doc.go.
//
// Typical wiring:
//
//	app := framework.New("svc", "1.0.0").Use(observability.Module)
//	if err := app.Init(ctx); err != nil { log.Fatal(err) }
//
//	obs := observability.FromApp(app)
//	obs.Logger().InfoContext(ctx, "ready")
//
// For HTTP services, wrap your router with the middleware sub-package:
//
//	import "github.com/godx-jp/godx-platform-framework/observability/middleware"
//	srv := &http.Server{Handler: middleware.HTTP(obs)(mux)}
package observability
