[← docs index](../README.md)

# observability module

The `observability` package gives a service one handle for structured logs (`log/slog`), distributed tracing (OpenTelemetry), and metrics (OpenTelemetry).

For the shared driver convention (auto vs opt-in registration, third-party drivers, layout rationale) see **[DRIVER_PATTERN](../DRIVER_PATTERN.md)** first — this document is the observability-specific reference.

## Package layout

```
observability/
├── doc.go              Package overview
├── module.go           framework.Module — wires Provider on app.Init
├── config.go           Config struct + LoadConfigFromEnv
├── provider.go         Provider — Logger() / Tracer() / Meter() / Channel()
├── channel.go          NewChannel module + channel registry
├── context.go          ContextWith{Provider,CorrelationID}, FromContext
├── register.go         Blank imports of light drivers (stdout/file/stack)
│
├── driver/             Public contract for custom drivers
├── drivers/            Built-in driver implementations
│   ├── stdout/         (light — auto)
│   ├── file/           (light — auto)
│   ├── stack/          (light — auto)
│   ├── otlp/           (heavy — opt-in blank import)
│   └── cloudwatch/     (heavy — opt-in; CloudWatch Logs v0.13.0)
└── middleware/         Optional HTTP middleware sub-package
```

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

fmt.Println("driver:", obs.Driver())
```

## What it gives you

| Signal | API | Wire format (depends on driver) |
|--------|-----|---------------------------------|
| Logs | `obs.Logger()` → `*slog.Logger` | JSON to stdout (`stdout`), local file (`file`), stdout + OTLP (`otlp`), fan-out (`stack`) |
| Traces | `obs.Tracer()` → `trace.Tracer` | OTLP gRPC/HTTP (`otlp`) or in-process (`stdout` / `file`) |
| Metrics | `obs.Meter()` → `metric.Meter` | OTLP gRPC/HTTP (`otlp`) or no-op (`stdout` / `file`) |
| HTTP middleware | `middleware.HTTP(obs)` (sub-package) | Span + correlation ID + per-request log |

## Auto-injected log attributes

Every record emitted via `*Logger.InfoContext` / `WarnContext` / etc. carries these for free:

| Attribute | Source |
|-----------|--------|
| `service`, `version`, `env` | `framework.App.Name()/.Version()`, `DEPLOYMENT_ENVIRONMENT` env |
| `trace_id`, `span_id` | Active OTel span on the context |
| `correlation_id` | Set by HTTP middleware or `ContextWithCorrelationID` |

If you call `Info` (no `Context` suffix), trace and correlation IDs are skipped — always prefer the `*Context` variants in handler / service code.

## Drivers — observability flavour of the driver pattern

| `OBSERVABILITY_DRIVER` | Status | Registration | What it does |
|------------------------|--------|--------------|--------------|
| `stdout` | stable | auto | slog JSON to stdout, in-process tracer (always-sample), no-op meter |
| `file` | stable | auto | slog JSON to a local file with Laravel-style rotation (`none` / `daily` / `size`), gzip, retention |
| `stack` | stable | auto | fan-out: every log record goes to N sub-drivers — Laravel `stack` channel |
| `otlp` | stable | **opt-in** (`_ "...drivers/otlp"`) | slog JSON to stdout + OTLP gRPC/HTTP for traces + metrics |
| `cloudwatch` | stub (0.4.x–0.5.x), full (0.6.0) | **opt-in** (`_ "...drivers/cloudwatch"`) | AWS CloudWatch Logs/Metrics + X-Ray |

For why some drivers are auto-registered and others opt-in, see [DRIVER_PATTERN — Light vs heavy drivers](../DRIVER_PATTERN.md#light-vs-heavy-drivers).

### stdout

Use for local dev, unit tests, ephemeral CI containers, any deployment where an orchestrator (Docker, Kubernetes, systemd-journald) is responsible for log collection.

- Logs: pretty JSON to stdout.
- Traces: spans created in-process and dropped at shutdown. You still see `trace_id` in logs.
- Metrics: counters/histograms run but are not exported.

### file

Use for bare-metal / VM deployments, zero-budget production where there is no log collector. Mirrors Laravel's `single` and `daily` channels.

| Env var | Type | Default | Purpose |
|---------|------|---------|---------|
| `OBSERVABILITY_LOG_FILE_PATH` | string | _required_ | Path to log file. Parent dir auto-created. |
| `OBSERVABILITY_LOG_FILE_ROTATION` | enum | `daily` | `none` (Laravel `single`) · `daily` · `size` |
| `OBSERVABILITY_LOG_FILE_MAX_SIZE_MB` | int | `100` | Size threshold |
| `OBSERVABILITY_LOG_FILE_MAX_AGE_DAYS` | int | `14` | Delete rotated files older than N days; `0` = forever |
| `OBSERVABILITY_LOG_FILE_MAX_BACKUPS` | int | `0` | Keep at most N rotated files; `0` = unlimited |
| `OBSERVABILITY_LOG_FILE_COMPRESS` | bool | `true` | Gzip rotated files |

Recipes (matches Laravel `config/logging.php`):

```bash
# Laravel `single` — append-only, no rotation
OBSERVABILITY_DRIVER=file \
OBSERVABILITY_LOG_FILE_PATH=./logs/app.log \
OBSERVABILITY_LOG_FILE_ROTATION=none

# Laravel `daily` (defaults — rotate at midnight, keep 14 days, gzip)
OBSERVABILITY_DRIVER=file \
OBSERVABILITY_LOG_FILE_PATH=./logs/app.log
```

Caveats:
- In containers, prefer `stdout`. The `file` driver inside a container couples log retention to the container lifecycle unless you mount a persistent volume.
- File rotation is in-process. If you also rotate externally via `logrotate(8)`, configure it with `copytruncate`.

### otlp (opt-in)

```go
import _ "github.com/godx-jp/godx-platform-framework/observability/drivers/otlp"
```

Use for any environment with an OTel-compatible receiver (godx-platform-observability, Datadog Agent, New Relic OTLP endpoint, Honeycomb, ...).

| Env var | Default | Purpose |
|---------|---------|---------|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | _required_ | `host:port` (no scheme), e.g. `otel-collector:4317` |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | `grpc` | `grpc` or `http` (== `http/protobuf`) |
| `OTEL_EXPORTER_OTLP_INSECURE` | `true` | Skip TLS verification (dev only) |
| `OBSERVABILITY_TRACE_SAMPLE_RATE` | `1.0` | Sample rate in `[0..1]` |

Logs are written to stdout (JSON) and are expected to be picked up out-of-process (Promtail / Fluent Bit / OTel Collector filelog receiver). This matches the standard container-log workflow and avoids dragging a third exporter into the binary.

### stack

Use when you want every log record to land in more than one place — for example stdout (so the orchestrator captures it) AND a local file (so an on-call engineer can `tail -f` over SSH) AND OTLP (so it ends up in Grafana / Datadog). Mirrors Laravel's `stack` log channel.

The stack driver instantiates each named sub-driver from the same Spec, dispatches every `slog.Record` to all of them, and joins their errors. Traces and metrics use the **first sub-driver only** — duplicating a span across exporters would produce double-counted distributed traces.

```bash
# Container + local safety net — every record to both
OBSERVABILITY_DRIVER=stack
OBSERVABILITY_STACK_DRIVERS=stdout,file
OBSERVABILITY_LOG_FILE_PATH=/var/log/app/app.log
```

**Per-sub minimum level** (`name:level` inline syntax — Laravel "info to stdout, warn+ to file" pattern):

```bash
OBSERVABILITY_DRIVER=stack
OBSERVABILITY_STACK_DRIVERS=stdout:info,file:warn
OBSERVABILITY_LOG_FILE_PATH=/var/log/app/app.log
```

`info` records land only in stdout; `warn` and above land in both. Omit `:level` to inherit the parent's `OBSERVABILITY_LOG_LEVEL`. Valid levels: `debug` · `info` · `warn` (or `warning`) · `error`. Unknown levels are rejected at construction with a clear error.

If `OBSERVABILITY_STACK_DRIVERS` includes a heavy driver (e.g. `otlp`), the consumer must still blank-import that driver package.

### cloudwatch (stub in 0.4.x–0.5.x — opt-in)

```go
import _ "github.com/godx-jp/godx-platform-framework/observability/drivers/cloudwatch"
```

Returns `cloudwatch.ErrNotImplemented` until 0.6.0. Designed as opt-in from day one so the future AWS-SDK-backed implementation is a drop-in upgrade. Tracked env vars: `AWS_REGION`, `OBSERVABILITY_CLOUDWATCH_LOG_GROUP`.

## HTTP middleware

The middleware sub-package is the optional HTTP integration. Import it only if you serve HTTP:

```go
import (
    "github.com/godx-jp/godx-platform-framework/observability"
    "github.com/godx-jp/godx-platform-framework/observability/middleware"
)

mux := http.NewServeMux()
mux.Handle("/api/", apiHandler)

obs := observability.FromApp(app)
srv := &http.Server{Addr: ":8080", Handler: middleware.HTTP(obs)(mux)}
```

For each request the middleware:

1. **Extracts trace context** from the incoming W3C `traceparent` header (or starts a new root span).
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

## Channels (Laravel-style named loggers)

Channels mirror Laravel's `Log::channel('name')->info(...)` API — multiple named loggers, each backed by its own driver and **its own minimum level**, selected per call. The primary channel comes from environment variables; additional channels can be declared two ways:

1. **In Go code** via [`NewChannel(name, cfg)`](../../observability/channel.go) — full programmatic control.
2. **Via env vars only** via [`ChannelsFromEnv()`](../../observability/channel.go) — zero Go code, pure 12-factor (matches Laravel `config/logging.php`).

Both can coexist. `NewChannel` is useful when channel config depends on runtime values; `ChannelsFromEnv` keeps `main.go` clean.

### Option 1 — declared in Go code

```go
app := framework.New("svc", "1.0.0").
    Use(observability.Module).                                    // primary from env
    Use(observability.NewChannel("audit", observability.Config{   // local file, warn+ only
        Driver:      observability.DriverFile,
        LogLevel:    slog.LevelWarn,                              // per-channel level
        LogFilePath: "/var/log/svc/audit.log",
    })).
    Use(observability.NewChannel("billing", observability.Config{ // separate OTLP collector
        Driver:       observability.DriverOTLP,
        OTLPEndpoint: "billing-collector:4317",
    }))
```

### Option 2 — declared via env vars only (`ChannelsFromEnv()`)

```go
app := framework.New("svc", "1.0.0").
    Use(observability.Module).
    Use(observability.ChannelsFromEnv())  // single line, no per-channel code
```

```bash
OBSERVABILITY_DRIVER=stdout
OBSERVABILITY_CHANNELS=audit,billing

# Each channel mirrors the primary env-var schema with the prefix
#   OBSERVABILITY_CHANNEL_<NAME>_
# Channel names are case-insensitive; hyphens normalise to underscores
# (so `audit-trail` reads OBSERVABILITY_CHANNEL_AUDIT_TRAIL_*).

OBSERVABILITY_CHANNEL_AUDIT_DRIVER=file
OBSERVABILITY_CHANNEL_AUDIT_LOG_LEVEL=warn                      # Laravel 'level' => 'warning'
OBSERVABILITY_CHANNEL_AUDIT_LOG_FILE_PATH=/var/log/svc/audit.log
OBSERVABILITY_CHANNEL_AUDIT_LOG_FILE_ROTATION=daily

OBSERVABILITY_CHANNEL_BILLING_DRIVER=otlp
OBSERVABILITY_CHANNEL_BILLING_OTLP_ENDPOINT=billing-collector:4317
OBSERVABILITY_CHANNEL_BILLING_OTLP_PROTOCOL=grpc

# Heavy drivers (otlp, cloudwatch) still need their blank import in main.go.
```

`ChannelsFromEnv()` is a no-op when `OBSERVABILITY_CHANNELS` is unset or empty, so it is safe to leave in `main` even for services that have not declared any extra channels yet.

Errors caught at startup: reserved name (`primary`), duplicate name, registration ordering (`ChannelsFromEnv()` must come after `Module`), unknown driver, missing required field per chosen driver.

Per-call selection inside a handler:

```go
func handler(w http.ResponseWriter, r *http.Request) {
    obs := observability.FromContext(r.Context())

    obs.Logger().InfoContext(r.Context(), "normal app log")                  // primary
    obs.Channel("audit").InfoContext(r.Context(), "user X did Y")            // file
    obs.Channel("billing").InfoContext(r.Context(), "payment", "amount", 100) // OTLP
}
```

Rules:

- `Channel("")` and `Channel("primary")` return the primary logger (== `Logger()`).
- An unknown channel name returns the primary logger and emits one warn line — calls never panic.
- Order matters: `observability.Module` must be `Use`d **before** any `NewChannel` so the primary provider exists at wire-up time. Wrong order returns a startup error.
- Channels are a **logging** concept. Traces and metrics always flow through the primary provider so distributed traces are not duplicated.

When you want every record to fan out to several destinations (same record everywhere), use the **stack driver** instead. Channels and stack compose: your primary channel can itself be a `stack`, and an extra channel can also be a `stack`.

Inspect:

```go
obs.Channels() // []string{"primary", "audit", "billing"}
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

## Performance notes

- The `contextHandler` wrap costs one map allocation per log record. Negligible for typical workloads.
- OTLP exporters batch by default (5s window, 512 record max). No-op when the queue is full so application code is never blocked.
- The stdout and file drivers are synchronous and unbatched — fine for dev / moderate prod traffic; not for hot paths emitting thousands of lines per second.

## See also

- [DRIVER_PATTERN](../DRIVER_PATTERN.md) — shared driver convention (applies to every module)
- [CONFIGURATION](../CONFIGURATION.md) — every observability env var
- [ARCHITECTURE](../ARCHITECTURE.md) — how the module plugs into framework.App
