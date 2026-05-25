# observability module

The `observability` package wires structured logging (`log/slog`), distributed tracing (OpenTelemetry traces), and metrics (OpenTelemetry metrics) into a service in a single line.

## Quick reference

```go
import (
    "github.com/godx-jp/godx-platform-framework/framework"
    "github.com/godx-jp/godx-platform-framework/observability"
)

app := framework.New("svc", "1.0.0").Use(observability.Module)
_ = app.Init(ctx)

obs := observability.FromApp(app)

obs.Logger().InfoContext(ctx, "started", "port", 8080)
ctx, span := obs.Tracer().Start(ctx, "operation-name")
counter, _ := obs.Meter().Int64Counter("requests_total")
counter.Add(ctx, 1)

// Inspect which driver is active (useful for liveness probes / debug pages).
fmt.Println("driver:", obs.Driver())
```

## What it gives you

| Signal | API | Wire format (depends on driver) |
|--------|-----|---------------------------------|
| Logs | `obs.Logger()` → `*slog.Logger` | JSON to stdout, or local file (`file` driver), or OTLP-correlated container-log workflow (`otlp` driver) |
| Traces | `obs.Tracer()` → `trace.Tracer` | OTLP gRPC/HTTP (`otlp` driver) or in-process (`stdout` / `file` drivers) |
| Metrics | `obs.Meter()` → `metric.Meter` | OTLP gRPC/HTTP (`otlp` driver) or no-op (`stdout` / `file` drivers) |
| HTTP middleware | `obs.Middleware(handler)` | Adds span + correlation ID + per-request log |

## Auto-injected log attributes

Every record emitted via `*Logger.InfoContext` / `WarnContext` / etc. gets these attributes for free when present in the context:

| Attribute | Source |
|-----------|--------|
| `service`, `version`, `env` | `framework.App.Name() / .Version()`, `DEPLOYMENT_ENVIRONMENT` env |
| `trace_id`, `span_id` | Active OTel span on the context |
| `correlation_id` | Set by HTTP middleware or `ContextWithCorrelationID` |

If you use `Info` (no `Context` suffix), trace and correlation IDs are skipped — always prefer the `*Context` variants in handler / service code.

## HTTP middleware

```go
mux := http.NewServeMux()
mux.Handle("/api/", apiHandler)
srv := &http.Server{Addr: ":8080", Handler: obs.Middleware(mux)}
```

For each request the middleware:

1. **Extracts trace context** from incoming W3C `traceparent` header (or starts a new root span).
2. **Reads or generates correlation ID** (`X-Correlation-ID`), stores it on `r.Context()`, echoes it on the response.
3. **Starts a server-kind span** named `METHOD /path`.
4. **Captures status code** via a wrapping `ResponseWriter`.
5. **Logs `http_request`** with `method`, `path`, `status`, `duration_ms`, `remote`.

Downstream handlers retrieve the provider via:

```go
func handler(w http.ResponseWriter, r *http.Request) {
    obs := observability.FromContext(r.Context())
    obs.Logger().InfoContext(r.Context(), "doing work")
}
```

## Context API

```go
ctx = observability.ContextWithProvider(ctx, p)         // explicit
ctx = observability.ContextWithCorrelationID(ctx, "id") // set CID

p   := observability.FromContext(ctx)                   // never nil
cid := observability.CorrelationIDFromContext(ctx)
```

## Using outside the framework

The `framework.Module` glue is a convenience; the SDK works standalone:

```go
cfg := observability.LoadConfigFromEnv()
cfg.ServiceName = "my-service"
cfg.ServiceVersion = "1.0.0"

p, err := observability.NewProvider(ctx, cfg)
if err != nil { ... }
defer p.Shutdown(ctx)
```

Useful for one-shot scripts or CLIs that don't want a long-running `App`.

## Choosing a logger style

- **`obs.Logger().InfoContext(ctx, msg, "key", val, ...)`** — preferred. Auto-injects trace and correlation IDs.
- **`slog.With("key", val).InfoContext(...)`** — for hot paths where you bind attributes once.
- **`slog.SetDefault(obs.Logger())`** — only if your transitive dependencies log via the stdlib default and you can't refactor them.

## Performance notes

- The `contextHandler` wrap costs one map allocation per log record (for the attributes). Negligible for typical workloads.
- OTLP exporters batch by default (5 s window, 512 record max). No-op when the queue is full to avoid blocking application code.
- The stdout and file drivers are synchronous and unbatched — fine for dev / moderate prod traffic, not for hot paths emitting thousands of log lines per second.
