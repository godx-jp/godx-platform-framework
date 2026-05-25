# Drivers

A **driver** is the in-process code that adapts the SDK's standard telemetry handles (`slog.Handler`, `trace.TracerProvider`, `metric.MeterProvider`) to a specific **destination** (a "backend" — Loki, CloudWatch, Datadog, a local file…). The application never imports a driver; it only sets `OBSERVABILITY_DRIVER`.

This is the same pattern as `database/sql` drivers in Go, or Laravel's filesystem / queue / cache drivers in PHP.

## Why the driver pattern

| Without it | With it |
|------------|---------|
| App code calls `cloudwatchlogs.PutLogEvents()` directly | App code calls `obs.Logger().Info(...)` |
| Swap CW → Datadog ⇒ rewrite logging across N services | Swap CW → Datadog ⇒ change one env var, redeploy |
| Different label conventions per backend | Standard OTel resource attrs everywhere |
| Vendor SDK dragged into the binary | OpenTelemetry SDK only |

The decision of *where telemetry goes* belongs to operators, not developers.

## Vocabulary

- **Driver** — the Go package that implements the telemetry plumbing (`drivers.Driver` interface). Examples: stdout, file, otlp, cloudwatch.
- **Backend** — the destination service that receives telemetry. Examples: Loki, Tempo, CloudWatch Logs, Datadog, New Relic, a local file on disk.

Selecting a driver picks the backend implicitly: `OBSERVABILITY_DRIVER=otlp` + `OTEL_EXPORTER_OTLP_ENDPOINT=otel-collector:4317` ships telemetry to whichever backend that collector forwards to.

## Available drivers (v0.2.x)

| `OBSERVABILITY_DRIVER` | Status | What it does | Dependencies |
|------------------------|--------|--------------|--------------|
| `stdout` | ✅ stable | slog JSON to stdout, in-process tracer (always sample), no-op meter | none beyond OTel SDK |
| `file` | ✅ stable | slog JSON to local file with Laravel-style rotation (`none` / `daily` / `size`), gzip, retention | `gopkg.in/natefinch/lumberjack.v2` |
| `otlp` | ✅ stable | slog JSON to stdout + OTLP gRPC/HTTP for traces + metrics | OTel OTLP exporters |
| `cloudwatch` | 🚧 stub | Returns `ErrCloudWatchNotImplemented` — full driver lands in 0.3.0 | (none yet) |

### `stdout`

Use for: local dev, unit tests, ephemeral CI containers, any deployment where an orchestrator (Docker, Kubernetes, systemd-journald) is responsible for log collection.

- Logs: pretty JSON to stdout.
- Traces: spans created in-process and dropped at shutdown (no exporter). You still see `trace_id` in logs.
- Metrics: counters/histograms run but are not exported.

### `file`

Use for: bare-metal / VM deployments, zero-budget production where there is no log collector. Mirrors Laravel's `single` and `daily` channels.

- Logs: JSON lines appended to `OBSERVABILITY_LOG_FILE_PATH`. Parent directory is auto-created.
- Traces: same as `stdout` (in-process, drops at shutdown; `trace_id` still appears in log records).
- Metrics: no-op.

| Env var | Type | Default | Purpose |
|---------|------|---------|---------|
| `OBSERVABILITY_LOG_FILE_PATH` | string | _required_ | Path to log file (absolute or relative). Parent dir auto-created. |
| `OBSERVABILITY_LOG_FILE_ROTATION` | enum | `daily` | `none` (Laravel `single`) · `daily` (Laravel `daily`) · `size` |
| `OBSERVABILITY_LOG_FILE_MAX_SIZE_MB` | int | `100` | Size threshold for `size` and `daily` rotation |
| `OBSERVABILITY_LOG_FILE_MAX_AGE_DAYS` | int | `14` | Delete rotated files older than N days; `0` = keep forever |
| `OBSERVABILITY_LOG_FILE_MAX_BACKUPS` | int | `0` | Keep at most N rotated files; `0` = unlimited |
| `OBSERVABILITY_LOG_FILE_COMPRESS` | bool | `true` | Gzip rotated files |

**Recipes** (matches Laravel `config/logging.php` channels):

```bash
# Laravel `single` — append-only, no rotation
OBSERVABILITY_DRIVER=file \
OBSERVABILITY_LOG_FILE_PATH=./logs/app.log \
OBSERVABILITY_LOG_FILE_ROTATION=none

# Laravel `daily` — rotate at midnight, keep 14 days, gzip
OBSERVABILITY_DRIVER=file \
OBSERVABILITY_LOG_FILE_PATH=./logs/app.log
# (daily / 14d / gzip are the defaults)

# Size-based rotation — useful for very chatty services
OBSERVABILITY_DRIVER=file \
OBSERVABILITY_LOG_FILE_PATH=./logs/app.log \
OBSERVABILITY_LOG_FILE_ROTATION=size \
OBSERVABILITY_LOG_FILE_MAX_SIZE_MB=50 \
OBSERVABILITY_LOG_FILE_MAX_BACKUPS=20
```

**Caveats**:
- In containers, prefer `stdout` so the orchestrator can capture logs. The `file` driver inside a container couples log retention to the container lifecycle unless you mount a persistent volume.
- File rotation is in-process. If you also rotate externally via `logrotate(8)`, configure it with `copytruncate` to avoid handle invalidation.

### `otlp`

Use for: any environment with an OTel-compatible receiver (godx-platform-observability, Datadog Agent, New Relic OTLP endpoint, Honeycomb, …).

| Env var | Purpose | Default |
|---------|---------|---------|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `host:port` (no scheme) | _required_ |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | `grpc` or `http` | `grpc` |
| `OTEL_EXPORTER_OTLP_INSECURE` | `true` skips TLS verify | `true` |
| `OBSERVABILITY_TRACE_SAMPLE_RATE` | sample rate `[0..1]` | `1.0` |

Logs are written to stdout (JSON) and are expected to be picked up out-of-process (Promtail / Fluent Bit / OTel Collector filelog receiver). This matches the standard container-log workflow and avoids dragging a third exporter into the binary.

### `cloudwatch` (stub in 0.2.x)

`NewProvider` returns `drivers.ErrCloudWatchNotImplemented`. The 0.3.0 release will use AWS ADOT exporters to push logs to CloudWatch Logs, metrics to CloudWatch Metrics, and traces to X-Ray. Tracked env vars (already accepted by `LoadConfigFromEnv`): `AWS_REGION`, `OBSERVABILITY_CLOUDWATCH_LOG_GROUP`.

## Writing a custom driver

A driver is anything that satisfies `drivers.Driver`:

```go
type Driver interface {
    LoggerHandler() slog.Handler
    TracerProvider() trace.TracerProvider
    MeterProvider() metric.MeterProvider
    Shutdown(ctx context.Context) error
}
```

Steps:

1. Add a file under `observability/drivers/yourdriver.go`.
2. Implement the four methods.
3. Add a `case "yourdriver":` to `drivers.New`.
4. Add a config field on `drivers.Spec` if needed.
5. Add an `OBSERVABILITY_DRIVER=yourdriver` test under `observability/`.

The `Driver` interface intentionally returns OTel `TracerProvider` / `MeterProvider` — that means your driver gets the entire OTel SDK for free. Most "new driver" implementations are 30-40 lines of OTel exporter wiring.

## Choosing a driver by environment

A common pattern:

```dockerfile
# Dockerfile (dev image)
ENV OBSERVABILITY_DRIVER=stdout
```

```yaml
# helm values.production.yaml
env:
  OBSERVABILITY_DRIVER: otlp
  OTEL_EXPORTER_OTLP_ENDPOINT: otel-collector.observability:4317
```

```ini
# bare-metal / systemd unit file — Laravel-style file logs
Environment="OBSERVABILITY_DRIVER=file"
Environment="OBSERVABILITY_LOG_FILE_PATH=/var/log/my-app/app.log"
Environment="OBSERVABILITY_LOG_FILE_ROTATION=daily"
Environment="OBSERVABILITY_LOG_FILE_MAX_AGE_DAYS=30"
```

```yaml
# helm values.aws.yaml (when 0.3.0 lands)
env:
  OBSERVABILITY_DRIVER: cloudwatch
  AWS_REGION: ap-northeast-1
  OBSERVABILITY_CLOUDWATCH_LOG_GROUP: /service/my-app
```

Same binary, four environments, zero application changes.
