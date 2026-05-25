# Configuration reference

All configuration is via environment variables (12-factor). The SDK ships with sensible defaults; in dev you typically need to set zero.

Naming rules:

- Variables that control this SDK are namespaced `OBSERVABILITY_*` — no abbreviations.
- Variables that are industry standards (OpenTelemetry, AWS) keep their canonical name so external tooling can read them too.

## Core

| Variable | Type | Default | Read by | Purpose |
|----------|------|---------|---------|---------|
| `DEPLOYMENT_ENVIRONMENT` | string | `dev` | observability | Populates `deployment.environment` resource attribute (OTel semconv) |

`SERVICE_NAME` and `SERVICE_VERSION` are **not** environment-driven — they are set by `framework.New(name, version)`. This is intentional: service identity is code, not config.

## Observability — common

| Variable | Type | Default | Purpose |
|----------|------|---------|---------|
| `OBSERVABILITY_DRIVER` | enum | `stdout` | Driver selector: `stdout` · `file` · `otlp` · `stack` · `cloudwatch` |
| `OBSERVABILITY_LOG_LEVEL` | enum | `info` | `debug` · `info` · `warn` · `error` |
| `OBSERVABILITY_TRACE_SAMPLE_RATE` | float | `1.0` | Sample rate in `[0..1]`; outside the range ⇒ always sample |

## Observability — stack driver

Required when `OBSERVABILITY_DRIVER=stack`. Every log record fans out to each named sub-driver in order. Each sub-driver inherits the rest of the env (so set `OBSERVABILITY_LOG_FILE_PATH` for a `file` sub-driver, `OTEL_EXPORTER_OTLP_ENDPOINT` for an `otlp` sub-driver, etc.).

| Variable | Type | Default | Purpose |
|----------|------|---------|---------|
| `OBSERVABILITY_STACK_DRIVERS` | comma list | _required_ | Sub-drivers in dispatch order, e.g. `stdout,file`. Whitespace tolerated. `stack` may not appear (no nesting). |

## Observability — file driver

Required when `OBSERVABILITY_DRIVER=file`. Laravel-style local file logging.

| Variable | Type | Default | Purpose |
|----------|------|---------|---------|
| `OBSERVABILITY_LOG_FILE_PATH` | string | _required_ | Path to log file (absolute or relative). Parent dir auto-created. |
| `OBSERVABILITY_LOG_FILE_ROTATION` | enum | `daily` | `none` (Laravel `single`) · `daily` · `size` |
| `OBSERVABILITY_LOG_FILE_MAX_SIZE_MB` | int | `100` | Size threshold (used by `size`; also caps `daily` files) |
| `OBSERVABILITY_LOG_FILE_MAX_AGE_DAYS` | int | `14` | Delete rotated files older than N days; `0` = forever |
| `OBSERVABILITY_LOG_FILE_MAX_BACKUPS` | int | `0` | Keep at most N rotated files; `0` = unlimited |
| `OBSERVABILITY_LOG_FILE_COMPRESS` | bool | `true` | Gzip rotated files |

## Observability — OTLP driver

Required when `OBSERVABILITY_DRIVER=otlp`. Names match the OpenTelemetry [environment variable spec](https://opentelemetry.io/docs/specs/otel/protocol/exporter/) so consumers can use any OTLP-aware tooling.

| Variable | Type | Default | Purpose |
|----------|------|---------|---------|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | string | _required_ | `host:port` (no scheme), e.g. `otel-collector:4317` |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | enum | `grpc` | `grpc` or `http` (== `http/protobuf`) |
| `OTEL_EXPORTER_OTLP_INSECURE` | bool | `true` | Skip TLS verification (dev only — set `false` for prod TLS endpoints) |

## Observability — CloudWatch driver (0.4.0+)

These are already read by `LoadConfigFromEnv` so consumers can set them today; the driver itself returns an error until 0.3.0.

| Variable | Type | Default | Purpose |
|----------|------|---------|---------|
| `AWS_REGION` | string | _unset_ | AWS region for CloudWatch + X-Ray endpoints |
| `OBSERVABILITY_CLOUDWATCH_LOG_GROUP` | string | _unset_ (driver derives from `service.name`) | Override CloudWatch log group name |

Standard AWS credential resolution (env, IRSA, EC2 metadata, `~/.aws/credentials`) applies once the 0.4.0 driver lands.

## HTTP middleware

| Constant | Default | Purpose |
|----------|---------|---------|
| `observability.CorrelationHeader` | `X-Correlation-ID` | Header read on requests / written on responses |

Configurable correlation header name is planned for 0.4.0.

## Defaults summary

```bash
# A service that opts into nothing gets:
SERVICE_NAME=<from framework.New>
SERVICE_VERSION=<from framework.New>
DEPLOYMENT_ENVIRONMENT=dev
OBSERVABILITY_DRIVER=stdout
OBSERVABILITY_LOG_LEVEL=info
OBSERVABILITY_TRACE_SAMPLE_RATE=1.0
```

## Local override file

A `.env` file at the service root works with most tools (Air, dotenv-cli, docker-compose). The SDK itself doesn't read `.env` — that's an explicit responsibility of the runtime.

Example `.env.example` shipped with a service:

```bash
DEPLOYMENT_ENVIRONMENT=dev

OBSERVABILITY_DRIVER=stdout       # stdout | file | otlp | stack | cloudwatch
OBSERVABILITY_LOG_LEVEL=info
OBSERVABILITY_TRACE_SAMPLE_RATE=1.0

# Required when OBSERVABILITY_DRIVER=stack (and set the sub-drivers' own vars too)
# OBSERVABILITY_STACK_DRIVERS=stdout,file

# Required when OBSERVABILITY_DRIVER=file
# OBSERVABILITY_LOG_FILE_PATH=./logs/app.log
# OBSERVABILITY_LOG_FILE_ROTATION=daily         # none | daily | size
# OBSERVABILITY_LOG_FILE_MAX_SIZE_MB=100
# OBSERVABILITY_LOG_FILE_MAX_AGE_DAYS=14
# OBSERVABILITY_LOG_FILE_MAX_BACKUPS=0
# OBSERVABILITY_LOG_FILE_COMPRESS=true

# Required when OBSERVABILITY_DRIVER=otlp
OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317
OTEL_EXPORTER_OTLP_PROTOCOL=grpc
OTEL_EXPORTER_OTLP_INSECURE=true

# Required when OBSERVABILITY_DRIVER=cloudwatch (0.4.0+)
# AWS_REGION=ap-northeast-1
# OBSERVABILITY_CLOUDWATCH_LOG_GROUP=/service/my-app
```
