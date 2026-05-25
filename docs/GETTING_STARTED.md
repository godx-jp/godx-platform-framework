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

## 3. Run with the dev backend (no infrastructure)

```bash
OBS_BACKEND=stdout go run .
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

Note: the SDK auto-injects `trace_id` and `span_id` from the active span — you didn't pass them in.

## 4. Switch to OTLP (push to godx-platform-observability)

Spin up the observability plane in a sibling repo:

```bash
git clone https://github.com/godx-jp/godx-platform-observability.git
cd godx-platform-observability && make up
```

Then run your service against it:

```bash
OBS_BACKEND=otlp \
OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317 \
go run .
```

Open Grafana at `http://localhost:3000` (default `admin/admin`) and find the trace under **Explore → Tempo**.

## 5. Add an HTTP middleware

```go
mux := http.NewServeMux()
mux.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
    obs := observability.FromContext(r.Context())
    obs.Logger().InfoContext(r.Context(), "answering")
    _, _ = w.Write([]byte("hi\n"))
})

srv := &http.Server{Addr: ":8080", Handler: obs.Middleware(mux)}
```

Every request now produces:
- A server-kind span for the request.
- A correlation ID (echoed in `X-Correlation-ID` response header).
- One `http_request` log line with `method`, `path`, `status`, `duration_ms`.

## Next steps

- **[OBSERVABILITY](./OBSERVABILITY.md)** — full SDK reference (logger, tracer, meter).
- **[BACKENDS](./BACKENDS.md)** — driver pattern, adding a new backend.
- **[CONFIGURATION](./CONFIGURATION.md)** — every env var.
