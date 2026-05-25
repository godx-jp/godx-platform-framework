// HTTP server example: spins up an http.Server wrapped with the framework
// observability middleware. Every request gets a trace + correlation ID and
// emits an http_request log line.
//
// Run:
//
//	OBS_BACKEND=stdout go run .
//	curl -i http://localhost:8080/hello
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/godx-jp/godx-platform-framework/framework"
	"github.com/godx-jp/godx-platform-framework/observability"
)

func main() {
	app := framework.New("http-example", "0.1.0").Use(observability.Module)

	ctx := context.Background()
	if err := app.Init(ctx); err != nil {
		panic(err)
	}

	obs := observability.FromApp(app)

	mux := http.NewServeMux()
	mux.HandleFunc("/hello", helloHandler)

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	srv := &http.Server{
		Addr:              addr,
		Handler:           obs.Middleware(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	app.OnShutdown(func(ctx context.Context) error {
		shutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutCtx)
	})

	obs.Logger().InfoContext(ctx, "http listening", "addr", addr)

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			obs.Logger().ErrorContext(ctx, "server crashed", "err", err)
		}
	}()

	if err := app.Run(ctx); err != nil {
		obs.Logger().Error("shutdown error", "err", err)
		os.Exit(1)
	}
}

func helloHandler(w http.ResponseWriter, r *http.Request) {
	obs := observability.FromContext(r.Context())

	_, span := obs.Tracer().Start(r.Context(), "compose-greeting")
	defer span.End()

	obs.Logger().InfoContext(r.Context(), "answering hello", "user_agent", r.UserAgent())
	_, _ = w.Write([]byte("hello\n"))
}
