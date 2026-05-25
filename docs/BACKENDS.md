# Backends

A backend is a **driver** that supplies the three concrete telemetry handles (`slog.Handler`, `trace.TracerProvider`, `metric.MeterProvider`) and a shutdown hook. The application never imports a backend; it only sets `OBS_BACKEND`.

This is the same pattern as `database/sql` drivers in Go, or Laravel's filesystem / queue / cache drivers in PHP.

## Why the driver pattern

| Without it | With it |
|------------|---------|
| App code calls `cloudwatchlogs.PutLogEvents()` directly | App code calls `obs.Logger().Info(...)` |
| Swap CW → Datadog ⇒ rewrite logging across N services | Swap CW → Datadog ⇒ change one env var, redeploy |
| Different label conventions per backend | Standard OTel resource attrs everywhere |
| Vendor SDK dragged into the binary | OpenTelemetry SDK only |

The decision of *where telemetry goes* belongs to operators, not developers.

## Available drivers (v0.1.0)

| `OBS_BACKEND` | Status | What it does | Dependencies |
|---------------|--------|--------------|--------------|
| `stdout` | ✅ stable | slog JSON to stdout, in-process tracer (always sample), no-op meter | none beyond OTel SDK |
| `otlp` | ✅ stable | slog JSON to stdout + OTLP gRPC/HTTP for traces + metrics | OTel OTLP exporters |
| `cloudwatch` | 🚧 stub | Returns `ErrCloudWatchNotImplemented` — full driver lands in 0.2.0 | (none yet) |

### `stdout`

Use for: local dev, unit tests, ephemeral CI containers.

- Logs: pretty JSON to stdout.
- Traces: spans created in-process and dropped at shutdown (no exporter). You still see `trace_id` in logs.
- Metrics: counters/histograms run but are not exported.

### `otlp`

Use for: any environment with an OTel-compatible receiver (godx-platform-observability, Datadog Agent, New Relic OTLP endpoint, Honeycomb, …).

| Env var | Purpose | Default |
|---------|---------|---------|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `host:port` (no scheme) | _required_ |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | `grpc` or `http` | `grpc` |
| `OTEL_EXPORTER_OTLP_INSECURE` | `true` skips TLS verify | `true` |
| `OBS_TRACE_SAMPLE` | sample rate `[0..1]` | `1.0` |

Logs are written to stdout (JSON) and are expected to be picked up out-of-process (Promtail / Fluent Bit / OTel Collector filelog receiver). This matches the standard container-log workflow and avoids dragging a third exporter into the binary.

### `cloudwatch` (stub in 0.1.0)

`NewProvider` returns `backends.ErrCloudWatchNotImplemented`. The 0.2.0 release will use AWS ADOT exporters to push logs to CloudWatch Logs, metrics to CloudWatch Metrics, and traces to X-Ray. Tracked env vars (already accepted by `LoadConfigFromEnv`): `AWS_REGION`, `OBS_LOG_GROUP`.

## Writing a custom driver

A driver is anything that satisfies `backends.Backend`:

```go
type Backend interface {
    LoggerHandler() slog.Handler
    TracerProvider() trace.TracerProvider
    MeterProvider() metric.MeterProvider
    Shutdown(ctx context.Context) error
}
```

Steps:

1. Add a file under `observability/backends/yourdriver.go`.
2. Implement the four methods.
3. Add a `case "yourdriver":` to `backends.New`.
4. Add a config field on `backends.Spec` if needed.
5. Add a `OBS_BACKEND=yourdriver` test under `observability/`.

The Backend interface intentionally returns OTel `TracerProvider` / `MeterProvider` — that means your driver gets the entire OTel SDK for free. Most "new backend" drivers are 30-40 lines of OTel exporter wiring.

## Choosing a backend by environment

A common pattern:

```dockerfile
# Dockerfile (dev image)
ENV OBS_BACKEND=stdout
```

```yaml
# helm values.production.yaml
env:
  OBS_BACKEND: otlp
  OTEL_EXPORTER_OTLP_ENDPOINT: otel-collector.observability:4317
```

```yaml
# helm values.aws.yaml (when 0.2.0 lands)
env:
  OBS_BACKEND: cloudwatch
  AWS_REGION: ap-northeast-1
  OBS_LOG_GROUP: /service/my-app
```

Same binary, three environments, zero application changes.
