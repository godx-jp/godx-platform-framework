[← docs index](./README.md)

# Getting started

Five minutes from zero to a Go service emitting traced, correlated, structured telemetry.

## 1. Add the SDK

```bash
mkdir hello-godx && cd hello-godx
go mod init hello-godx
go get github.com/godx-jp/godx-platform-framework@latest
```

## 2. Write the smallest possible service

`main.go`:

```go
package main

import (
    "context"

    "github.com/godx-jp/godx-platform-framework/framework"
    "github.com/godx-jp/godx-platform-framework/observability"
)

func main() {
    app := framework.New("hello-godx", "0.1.0").Use(observability.Module)

    if err := app.Init(context.Background()); err != nil {
        panic(err)
    }
    defer app.Shutdown(context.Background())

    obs := observability.FromApp(app)
    ctx, span := obs.Tracer().Start(context.Background(), "say-hello")
    defer span.End()

    obs.Logger().InfoContext(ctx, "hello world", "answer", 42)
}
```

## 3. Run with the stdout driver (no infrastructure)

```bash
OBSERVABILITY_DRIVER=stdout go run .
```

Expected output (formatted for readability):

```json
{
  "time":     "2026-05-25T15:30:00+09:00",
  "level":    "INFO",
  "msg":      "hello world",
  "service":  "hello-godx",
  "version":  "0.1.0",
  "env":      "dev",
  "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
  "span_id":  "00f067aa0ba902b7",
  "answer":   42
}
```

The SDK auto-injects `trace_id` and `span_id` from the active span — you didn't pass them in.

## 4. Switch to the file driver (Laravel-style local logs)

For bare-metal / VM deployments where there is no log collector. Mirrors Laravel's `daily` channel.

```bash
OBSERVABILITY_DRIVER=file \
OBSERVABILITY_LOG_FILE_PATH=./logs/app.log \
go run .

tail -f logs/app.log
```

Defaults: daily rotation, 14-day retention, gzip on rotation. See [modules/observability — file](./modules/observability.md#file) for tuning.

## 5. Switch to OTLP (push to godx-platform-observability)

OTLP is a **heavy driver** — it's opt-in via a blank import to keep binaries small for services that don't need it:

```go
import (
    "github.com/godx-jp/godx-platform-framework/framework"
    "github.com/godx-jp/godx-platform-framework/observability"
    _ "github.com/godx-jp/godx-platform-framework/observability/drivers/otlp" // opt-in
)
```

Spin up the observability plane in a sibling repo:

```bash
git clone https://github.com/godx-jp/godx-platform-observability.git
cd godx-platform-observability && make up
```

Then run your service against it:

```bash
OBSERVABILITY_DRIVER=otlp \
OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317 \
go run .
```

Open Grafana at `http://localhost:3000` (default `admin/admin`) and find the trace under **Explore → Tempo**.

If you forget the blank import, you'll get a clear runtime error:

```
observability/driver: "otlp" not registered (have: [file stack stdout]) — heavy drivers require an explicit blank import, e.g. _ "github.com/godx-jp/godx-platform-framework/observability/drivers/otlp"
```

## 6. Add HTTP middleware

The HTTP middleware lives in its own sub-package so non-HTTP callers don't transitively import `net/http`:

```go
import (
    "github.com/godx-jp/godx-platform-framework/observability"
    "github.com/godx-jp/godx-platform-framework/observability/middleware"
)

mux := http.NewServeMux()
mux.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
    obs := observability.FromContext(r.Context())
    obs.Logger().InfoContext(r.Context(), "answering")
    _, _ = w.Write([]byte("hi\n"))
})

obs := observability.FromApp(app)
srv := &http.Server{Addr: ":8080", Handler: middleware.HTTP(obs)(mux)}
```

Every request now produces:

- A server-kind span for the request.
- A correlation ID (echoed in `X-Correlation-ID` response header).
- One `http_request` log line with `method`, `path`, `status`, `duration_ms`.

## Next steps

- **[ARCHITECTURE](./ARCHITECTURE.md)** — how the framework backbone and modules fit together.
- **[DRIVER_PATTERN](./DRIVER_PATTERN.md)** — shared convention for swappable backends (applies to every module).
- **[modules/observability](./modules/observability.md)** — full SDK reference (logger, tracer, meter, channels, drivers).
- **[CONFIGURATION](./CONFIGURATION.md)** — every env var.
