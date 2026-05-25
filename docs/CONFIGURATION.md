[← docs index](./README.md)

# Configuration reference

All configuration is via environment variables (12-factor). The SDK ships with sensible defaults; in dev you typically need to set zero.

Naming rules:

- Variables that control this SDK are namespaced by module: `OBSERVABILITY_*`, `STORAGE_*`, etc. — no abbreviations.
- Variables that are industry standards (OpenTelemetry, AWS) keep their canonical name so external tooling can read them too.

## Core

| Variable | Type | Default | Read by | Purpose |
|----------|------|---------|---------|---------|
| `DEPLOYMENT_ENVIRONMENT` | string | `dev` | observability | Populates `deployment.environment` resource attribute (OTel semconv) |

`SERVICE_NAME` and `SERVICE_VERSION` are **not** environment-driven — they are set by `framework.New(name, version)`. This is intentional: service identity is code, not config.

## Observability — common

| Variable | Type | Default | Purpose |
|----------|------|---------|---------|
| `OBSERVABILITY_DRIVER` | enum | `stdout` | Driver selector: `stdout` · `file` · `stack` · `otlp` · `cloudwatch` |
| `OBSERVABILITY_LOG_LEVEL` | enum | `info` | `debug` · `info` · `warn` · `error` |
| `OBSERVABILITY_TRACE_SAMPLE_RATE` | float | `1.0` | Sample rate in `[0..1]`; outside the range ⇒ always sample |

## Observability — light drivers (auto-registered)

`stdout`, `file`, and `stack` are registered automatically when the `observability` package is imported. No blank import needed.

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

## Observability — stack driver

Required when `OBSERVABILITY_DRIVER=stack`. Every log record fans out to each named sub-driver in order. Each sub-driver inherits the rest of the env (so set `OBSERVABILITY_LOG_FILE_PATH` for a `file` sub-driver, `OTEL_EXPORTER_OTLP_ENDPOINT` for an `otlp` sub-driver, etc.).

| Variable | Type | Default | Purpose |
|----------|------|---------|---------|
| `OBSERVABILITY_STACK_DRIVERS` | comma list | _required_ | Sub-drivers in dispatch order, e.g. `stdout,file`. Each entry may carry an optional per-sub minimum level via inline `name:level` syntax — e.g. `stdout:info,file:warn`. Valid levels: `debug` · `info` · `warn` · `error`. Without `:level` the sub inherits `OBSERVABILITY_LOG_LEVEL`. `stack` may not appear (no nesting). |

If a sub-driver name refers to a heavy driver (`otlp`, `cloudwatch`), the consumer must blank-import that driver package as well.

## Observability — env-driven extra channels

Declare any number of named channels (Laravel `Log::channel('name')` API) purely via env vars, no Go code:

| Variable | Type | Default | Purpose |
|----------|------|---------|---------|
| `OBSERVABILITY_CHANNELS` | comma list | _unset_ | Channel names to register with `ChannelsFromEnv()`. `primary` is reserved; duplicates rejected. |
| `OBSERVABILITY_CHANNEL_<NAME>_DRIVER` | enum | `stdout` | Driver for this channel — same set as `OBSERVABILITY_DRIVER` |
| `OBSERVABILITY_CHANNEL_<NAME>_LOG_LEVEL` | enum | `info` | Per-channel minimum level (Laravel `'level' => 'warning'`) |
| `OBSERVABILITY_CHANNEL_<NAME>_TRACE_SAMPLE_RATE` | float | `1.0` | Sample rate `[0..1]` |
| `OBSERVABILITY_CHANNEL_<NAME>_LOG_FILE_PATH` | string | _required for file_ | File path for this channel |
| `OBSERVABILITY_CHANNEL_<NAME>_LOG_FILE_ROTATION` | enum | `daily` | `none` · `daily` · `size` |
| `OBSERVABILITY_CHANNEL_<NAME>_LOG_FILE_MAX_SIZE_MB` | int | `100` | |
| `OBSERVABILITY_CHANNEL_<NAME>_LOG_FILE_MAX_AGE_DAYS` | int | `14` | |
| `OBSERVABILITY_CHANNEL_<NAME>_LOG_FILE_MAX_BACKUPS` | int | `0` | |
| `OBSERVABILITY_CHANNEL_<NAME>_LOG_FILE_COMPRESS` | bool | `true` | |
| `OBSERVABILITY_CHANNEL_<NAME>_OTLP_ENDPOINT` | string | _required for otlp_ | `host:port` (per-channel; the global `OTEL_EXPORTER_OTLP_*` is NOT inherited because it is intentionally a singleton) |
| `OBSERVABILITY_CHANNEL_<NAME>_OTLP_PROTOCOL` | enum | `grpc` | `grpc` or `http` |
| `OBSERVABILITY_CHANNEL_<NAME>_OTLP_INSECURE` | bool | `true` | |
| `OBSERVABILITY_CHANNEL_<NAME>_STACK_DRIVERS` | comma list | _required for stack_ | Same `name[:level]` syntax as the global one |
| `OBSERVABILITY_CHANNEL_<NAME>_AWS_REGION` | string | _unset_ | Per-channel AWS region (for `cloudwatch` driver, 0.6.0+) |
| `OBSERVABILITY_CHANNEL_<NAME>_CLOUDWATCH_LOG_GROUP` | string | _unset_ | Per-channel CloudWatch log group |

Channel name normalisation: case-insensitive; hyphens and spaces convert to underscores. `audit-trail` ⇒ `OBSERVABILITY_CHANNEL_AUDIT_TRAIL_*`.

Wiring:

```go
.Use(observability.Module).
.Use(observability.ChannelsFromEnv())   // must come after Module
```

`ChannelsFromEnv()` is a no-op when `OBSERVABILITY_CHANNELS` is unset, so it is safe to leave in `main` permanently.

## Observability — heavy drivers (opt-in)

Heavy drivers require an explicit blank import in consumer code; if you select a heavy driver without importing it the SDK fails fast with `"<name>" not registered`.

### otlp

```go
import _ "github.com/godx-jp/godx-platform-framework/observability/drivers/otlp"
```

Required when `OBSERVABILITY_DRIVER=otlp`. Names match the OpenTelemetry [environment variable spec](https://opentelemetry.io/docs/specs/otel/protocol/exporter/) so consumers can use any OTLP-aware tooling.

| Variable | Type | Default | Purpose |
|----------|------|---------|---------|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | string | _required_ | `host:port` (no scheme), e.g. `otel-collector:4317` |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | enum | `grpc` | `grpc` or `http` (== `http/protobuf`) |
| `OTEL_EXPORTER_OTLP_INSECURE` | bool | `true` | Skip TLS verification (dev only — set `false` for prod TLS endpoints) |

### cloudwatch (stub through v0.7.x, full in v0.8.0)

```go
import _ "github.com/godx-jp/godx-platform-framework/observability/drivers/cloudwatch"
```

Tracked env vars are already accepted by `LoadConfigFromEnv` so consumers can set them today; the driver itself returns `cloudwatch.ErrNotImplemented` until v0.8.0.

| Variable | Type | Default | Purpose |
|----------|------|---------|---------|
| `AWS_REGION` | string | _unset_ | AWS region for CloudWatch + X-Ray endpoints |
| `OBSERVABILITY_CLOUDWATCH_LOG_GROUP` | string | _unset_ (driver derives from `service.name`) | Override CloudWatch log group name |

Standard AWS credential resolution (env, IRSA, EC2 metadata, `~/.aws/credentials`) applies once the driver lands in v0.8.0.

## Storage — common

The storage module loads zero or more named disks from the environment. With nothing set, you get a single `local` disk rooted at `./storage/app/private` with private visibility — matching Laravel's bare-install `local` disk.

| Variable | Type | Default | Purpose |
|----------|------|---------|---------|
| `STORAGE_DEFAULT_DISK` | string | `local` | Name returned by `Manager.Default()` / `mgr.DefaultName()` |
| `STORAGE_DISKS` | comma list | `<value of STORAGE_DEFAULT_DISK>` | Disks to register; each name must have at least the per-disk `DRIVER` env var set when not `local` |

## Storage — per-disk env vars

For each disk `<NAME>` listed in `STORAGE_DISKS`, the module reads env vars prefixed `STORAGE_DISK_<NAME>_`. Disk-name normalisation matches observability channels: case-insensitive; hyphens and spaces convert to underscores (`avatars-public` ⇒ `STORAGE_DISK_AVATARS_PUBLIC_*`).

| Variable | Type | Default | Driver scope | Purpose |
|----------|------|---------|--------------|---------|
| `STORAGE_DISK_<NAME>_DRIVER` | enum | `local` | all | `local` · `memory` · `s3` · `minio` · `gcs` · `azure` |
| `STORAGE_DISK_<NAME>_ROOT` | string | `./storage/app/private` (for `local`) | local | Filesystem root path. Laravel-faithful default — set explicitly to override. The conventional matching "public" disk is rooted at `./storage/app/public`. |
| `STORAGE_DISK_<NAME>_VISIBILITY` | enum | `private` | local + cloud | Default visibility for writes — `public` or `private` |
| `STORAGE_DISK_<NAME>_PUBLIC_URL` | string | _unset_ | local + cloud | Base URL for `disk.URL(key)` — e.g. `https://cdn.example.com` |
| `STORAGE_DISK_<NAME>_BUCKET` | string | _required for cloud_ | s3 · gcs · azure · minio | Bucket / container name |
| `STORAGE_DISK_<NAME>_REGION` | string | _unset_ | s3 · minio | Cloud region — e.g. `ap-northeast-1` |
| `STORAGE_DISK_<NAME>_ENDPOINT` | string | _unset_ | s3 · minio · gcs · azure | Custom endpoint URL. Required for MinIO. **Required for Azure** (service URL: `https://<account>.blob.core.windows.net`). For GCS, set when targeting an emulator (e.g. `fake-gcs-server`) to switch the client to no-auth mode |
| `STORAGE_DISK_<NAME>_USE_PATH_STYLE` | bool | `false` (S3) / `true` (MinIO) | s3 · minio | Force path-style addressing |
| `STORAGE_DISK_<NAME>_ACCESS_KEY` | string | _unset_ (resolved by SDK) | s3 · minio · azure | Explicit access key. For Azure, this is the **storage account name** (paired with `SECRET_KEY` = account key, required for SAS issuance) |
| `STORAGE_DISK_<NAME>_SECRET_KEY` | string | _unset_ | s3 · minio · azure | Explicit secret key. For Azure, the **storage account key** |
| `STORAGE_DISK_<NAME>_SESSION_TOKEN` | string | _unset_ | s3 | AWS STS session token |

## Storage — heavy drivers (opt-in)

Heavy drivers (`s3`, `minio`, `gcs`, `azure`) require an explicit blank import in consumer code; the SDK fails fast at boot with a clear hint if you select a heavy driver without importing its package.

```go
import _ "github.com/godx-jp/godx-platform-framework/storage/drivers/s3"
import _ "github.com/godx-jp/godx-platform-framework/storage/drivers/minio"
```

| Driver | Status (v0.6.2) | Required env keys | Notes |
|--------|-----------------|--------------------|-------|
| `s3`   | **stable** | `BUCKET`, `REGION` | Credentials via default AWS chain (env → `~/.aws/credentials` → IRSA → EC2 IMDS); override with `ACCESS_KEY` / `SECRET_KEY` |
| `minio` | **stable** | `BUCKET`, `ENDPOINT` | Always path-style; `REGION` defaults to `us-east-1`; supply credentials via `ACCESS_KEY` / `SECRET_KEY` |
| `gcs`  | **stable** | `BUCKET` | Application Default Credentials (`GOOGLE_APPLICATION_CREDENTIALS`, `gcloud auth application-default login`, Workload Identity). Setting `ENDPOINT` switches to emulator mode (`fake-gcs-server`). `SignedURL` requires a service-account JSON key |
| `azure` | **stable** | `BUCKET` (container), `ENDPOINT` (service URL) | Shared-key auth (`ACCESS_KEY` = account name + `SECRET_KEY` = account key) is the preferred mode — only that combination can issue SAS via `TemporaryURL`. Without it, the driver falls back to `DefaultAzureCredential` |

## HTTP middleware

| Constant | Default | Purpose |
|----------|---------|---------|
| `middleware.CorrelationHeader` | `X-Correlation-ID` | Header read on requests / written on responses |

The constant lives in the `observability/middleware` sub-package. Configurable correlation header name is planned for a later release.

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

OBSERVABILITY_DRIVER=stdout       # stdout | file | stack | otlp | cloudwatch
OBSERVABILITY_LOG_LEVEL=info
OBSERVABILITY_TRACE_SAMPLE_RATE=1.0

# Required when OBSERVABILITY_DRIVER=stack — optional per-sub level via `name:level`
# OBSERVABILITY_STACK_DRIVERS=stdout:info,file:warn

# Required when OBSERVABILITY_DRIVER=file (or used as stack sub-driver)
# OBSERVABILITY_LOG_FILE_PATH=./logs/app.log
# OBSERVABILITY_LOG_FILE_ROTATION=daily         # none | daily | size
# OBSERVABILITY_LOG_FILE_MAX_SIZE_MB=100
# OBSERVABILITY_LOG_FILE_MAX_AGE_DAYS=14
# OBSERVABILITY_LOG_FILE_MAX_BACKUPS=0
# OBSERVABILITY_LOG_FILE_COMPRESS=true

# Required when OBSERVABILITY_DRIVER=otlp (consumer must blank-import drivers/otlp)
# OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317
# OTEL_EXPORTER_OTLP_PROTOCOL=grpc
# OTEL_EXPORTER_OTLP_INSECURE=true

# Required when OBSERVABILITY_DRIVER=cloudwatch (0.6.0+; blank-import drivers/cloudwatch)
# AWS_REGION=ap-northeast-1
# OBSERVABILITY_CLOUDWATCH_LOG_GROUP=/service/my-app

# --- Extra channels (Laravel-style, declared purely via env) ---
# OBSERVABILITY_CHANNELS=audit,billing
#
# OBSERVABILITY_CHANNEL_AUDIT_DRIVER=file
# OBSERVABILITY_CHANNEL_AUDIT_LOG_LEVEL=warn
# OBSERVABILITY_CHANNEL_AUDIT_LOG_FILE_PATH=/var/log/svc/audit.log
#
# OBSERVABILITY_CHANNEL_BILLING_DRIVER=otlp
# OBSERVABILITY_CHANNEL_BILLING_OTLP_ENDPOINT=billing-collector:4317

# --- Storage (Laravel-style multi-disk; zero env vars ⇒ single local disk) ---
# STORAGE_DEFAULT_DISK=local
# STORAGE_DISKS=local,public,uploads
#
# # Default Laravel local disk
# STORAGE_DISK_LOCAL_DRIVER=local
# STORAGE_DISK_LOCAL_ROOT=./storage/app/private
# STORAGE_DISK_LOCAL_VISIBILITY=private
#
# # Conventional "public" disk (symlinked from web root in production)
# STORAGE_DISK_PUBLIC_DRIVER=local
# STORAGE_DISK_PUBLIC_ROOT=./storage/app/public
# STORAGE_DISK_PUBLIC_VISIBILITY=public
# STORAGE_DISK_PUBLIC_PUBLIC_URL=http://localhost:8000/storage
#
# # AWS S3 (require: import _ ".../storage/drivers/s3")
# STORAGE_DISK_UPLOADS_DRIVER=s3
# STORAGE_DISK_UPLOADS_BUCKET=my-uploads
# STORAGE_DISK_UPLOADS_REGION=ap-northeast-1
# # Optional: STORAGE_DISK_UPLOADS_PUBLIC_URL=https://cdn.example.com
#
# # MinIO / S3-compatible (require: import _ ".../storage/drivers/minio")
# # STORAGE_DISK_DEVCACHE_DRIVER=minio
# # STORAGE_DISK_DEVCACHE_BUCKET=uploads
# # STORAGE_DISK_DEVCACHE_ENDPOINT=http://localhost:9000
# # STORAGE_DISK_DEVCACHE_ACCESS_KEY=minioadmin
# # STORAGE_DISK_DEVCACHE_SECRET_KEY=minioadmin
```
