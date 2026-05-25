# Configuration reference

All configuration is via environment variables (12-factor). The SDK ships with sensible defaults; in dev you typically need to set zero.

## Core

| Variable | Type | Default | Read by | Purpose |
|----------|------|---------|---------|---------|
| `DEPLOYMENT_ENVIRONMENT` | string | `dev` | observability | Populates `deployment.environment` resource attribute (OTel semconv) |

`SERVICE_NAME` and `SERVICE_VERSION` are **not** environment-driven — they are set by `framework.New(name, version)`. This is intentional: service identity is code, not config.

## Observability — common

| Variable | Type | Default | Purpose |
|----------|------|---------|---------|
| `OBS_BACKEND` | enum | `stdout` | Driver selector: `stdout` · `file` · `otlp` · `cloudwatch` |
| `OBS_LOG_LEVEL` | enum | `info` | `debug` · `info` · `warn` · `error` |
| `OBS_TRACE_SAMPLE` | float | `1.0` | Sample rate in `[0..1]`; outside the range ⇒ always sample |

## Observability — file driver

Required when `OBS_BACKEND=file`. Laravel-style local file logging.

| Variable | Type | Default | Purpose |
|----------|------|---------|---------|
| `OBS_LOG_FILE` | string | _required_ | Path to log file (absolute or relative). Parent dir auto-created. |
| `OBS_LOG_ROTATE` | enum | `daily` | `none` (Laravel `single`) · `daily` · `size` |
| `OBS_LOG_MAX_SIZE_MB` | int | `100` | Size threshold (used by `size`; also caps `daily` files) |
| `OBS_LOG_MAX_AGE_DAYS` | int | `14` | Delete rotated files older than N days; `0` = forever |
| `OBS_LOG_MAX_BACKUPS` | int | `0` | Keep at most N rotated files; `0` = unlimited |
| `OBS_LOG_COMPRESS` | bool | `true` | Gzip rotated files |

## Observability — OTLP driver

Required when `OBS_BACKEND=otlp`. Names match the OpenTelemetry [environment variable spec](https://opentelemetry.io/docs/specs/otel/protocol/exporter/) so consumers can use any OTLP-aware tooling.

| Variable | Type | Default | Purpose |
|----------|------|---------|---------|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | string | _required_ | `host:port` (no scheme), e.g. `otel-collector:4317` |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | enum | `grpc` | `grpc` or `http` (== `http/protobuf`) |
| `OTEL_EXPORTER_OTLP_INSECURE` | bool | `true` | Skip TLS verification (dev only — set `false` for prod TLS endpoints) |

## Observability — CloudWatch driver (0.3.0+)

These are already read by `LoadConfigFromEnv` so consumers can set them today; the driver itself returns an error until 0.2.0.

| Variable | Type | Default | Purpose |
|----------|------|---------|---------|
| `AWS_REGION` | string | _unset_ | AWS region for CloudWatch + X-Ray endpoints |
| `OBS_LOG_GROUP` | string | _unset_ (driver derives from `service.name`) | Override CloudWatch log group name |

Standard AWS credential resolution (env, IRSA, EC2 metadata, `~/.aws/credentials`) applies once the 0.3.0 driver lands.

## HTTP middleware

| Constant | Default | Purpose |
|----------|---------|---------|
| `observability.CorrelationHeader` | `X-Correlation-ID` | Header read on requests / written on responses |

Configurable correlation header name is planned for 0.2.0.

## Defaults summary table

```bash
# A service that opts into nothing gets:
SERVICE_NAME=<from framework.New>
SERVICE_VERSION=<from framework.New>
DEPLOYMENT_ENVIRONMENT=dev
OBS_BACKEND=stdout
OBS_LOG_LEVEL=info
OBS_TRACE_SAMPLE=1.0
```

## Local override file

A `.env` file at the service root works with most tools (Air, dotenv-cli, docker-compose). The SDK itself doesn't read `.env` — that's an explicit responsibility of the runtime.

Example `.env.example` shipped with a service:

```bash
DEPLOYMENT_ENVIRONMENT=dev

OBS_BACKEND=stdout       # stdout | file | otlp | cloudwatch
OBS_LOG_LEVEL=info
OBS_TRACE_SAMPLE=1.0

# Required when OBS_BACKEND=file
# OBS_LOG_FILE=./logs/app.log
# OBS_LOG_ROTATE=daily         # none | daily | size
# OBS_LOG_MAX_SIZE_MB=100
# OBS_LOG_MAX_AGE_DAYS=14
# OBS_LOG_MAX_BACKUPS=0
# OBS_LOG_COMPRESS=true

# Required when OBS_BACKEND=otlp
OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317
OTEL_EXPORTER_OTLP_PROTOCOL=grpc
OTEL_EXPORTER_OTLP_INSECURE=true

# Required when OBS_BACKEND=cloudwatch (0.3.0+)
# AWS_REGION=ap-northeast-1
# OBS_LOG_GROUP=/service/my-app
```
