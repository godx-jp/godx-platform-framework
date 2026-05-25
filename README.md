# godx-platform-framework

> **Opinionated Go SDK by godx** — modular, OpenTelemetry-native, backend-agnostic.
> Write once, swap backends (godx-platform-observability ↔ AWS CloudWatch ↔ Datadog ↔ …) by changing one env var.

[![Version](https://img.shields.io/badge/version-0.8.5-blue.svg)](./CHANGELOG.md)
[![License](https://img.shields.io/badge/license-Apache_2.0-green.svg)](./LICENSE)
[![Maintainer](https://img.shields.io/badge/by-godx-black.svg)](#)
[![Go](https://img.shields.io/badge/go-1.23+-00ADD8.svg)](https://go.dev)

## Who is this for

Any Go team that wants production-grade infrastructure without writing it from scratch — and without being locked into one vendor. v0.6 ships the observability and storage modules; cache, queue, and httpx are on the roadmap and follow the same conventions.

| You have… | This gives you… |
|-----------|-----------------|
| A new service | A `framework.New(...).Use(observability.Module).Run(ctx)` boilerplate |
| Many services across teams | A shared SDK with the same log format, the same trace IDs, the same metric names |
| **Zero budget / bare metal** | **Log to a local file (Laravel-style `single` / `daily` rotation)** |
| Self-hosted observability | Point at `godx-platform-observability` via OTLP |
| AWS-only budget | Point at CloudWatch (one env var change, opt-in import) |
| Datadog / New Relic licence | Point at their OTLP endpoint (one env var change) |
| All of the above (dev=stdout, staging=file, prod=OTLP) | One binary, env-driven backend selection |

Zero application code change between backends.

## Quick start

```bash
go get github.com/godx-jp/godx-platform-framework@latest
```

```go
package main

import (
    "context"
    "github.com/godx-jp/godx-platform-framework/framework"
    "github.com/godx-jp/godx-platform-framework/observability"
)

func main() {
    app := framework.New("my-app", "1.0.0").
        Use(observability.Module)

    if err := app.Run(context.Background()); err != nil {
        panic(err)
    }
}
```

```bash
# Dev — pretty JSON logs to stdout, no infra needed
OBSERVABILITY_DRIVER=stdout go run .

# Zero-budget — append JSON lines to a local file (Laravel `single` channel)
OBSERVABILITY_DRIVER=file \
OBSERVABILITY_LOG_FILE_PATH=./logs/app.log \
OBSERVABILITY_LOG_FILE_ROTATION=none \
go run .

# Production on bare metal — daily-rotated file, keep 14 days, gzip old
OBSERVABILITY_DRIVER=file \
OBSERVABILITY_LOG_FILE_PATH=/var/log/app/app.log \
go run .
```

For OTLP / CloudWatch (heavy drivers), add a blank import to your `main` so the driver registers itself:

```go
import _ "github.com/godx-jp/godx-platform-framework/observability/drivers/otlp"
```

```bash
# Self-hosted — push to godx-platform-observability
OBSERVABILITY_DRIVER=otlp \
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4317 \
go run .

# Datadog — same OTLP path, just different endpoint
OBSERVABILITY_DRIVER=otlp \
OTEL_EXPORTER_OTLP_ENDPOINT=http://datadog-agent:4317 \
go run .
```

## Modules

| Module | Status | Purpose |
|--------|--------|---------|
| `framework` | stable | App backbone — module registration, lifecycle, graceful shutdown |
| `observability` | stable | Logs (slog JSON) + traces (OTel) + metrics (OTel) + Laravel-style channels |
| `storage` | stable (v0.6.x) | object storage — local · memory · s3 · minio · gcs · azure (all six stable) |
| `cache` | stable (v0.7.0) | Laravel-style cache — memory · file · redis (DB-backed cache intentionally out of scope) |
| `config` | stable (v0.8.0) | layered configuration repository — env · file (yaml/json/toml) · static · remote (heavy, roadmap) |
| `events` | stable (v0.8.1) | sync + async dispatcher, wildcard listeners (`user.*`, `*.deleted`) |
| `hashing` | stable (v0.8.2) | bcrypt · argon2id · scrypt — Laravel `Hash::` parity with `NeedsRehash` |
| `encryption` | stable (v0.8.3) | aesgcm · chacha20poly1305 — versioned key rotation, Laravel `Crypt::` parity |
| `pipeline` | stable (v0.8.4) | composable middleware chain — Laravel `Pipeline` parity, generic over T, net/http compat |
| `secrets` | stable (v0.8.5) | env · file · vault · gcpsm · awssm — uniform Get/Put/Forget |
| `validation` | roadmap (v0.9.0) | struct-tag DSL, pluggable rule registry, i18n templates |
| `httpclient` | roadmap (v0.9.1) | stdlib + resilient, OTel auto-instrumentation |
| `ratelimit` | roadmap (v0.9.2) | memory + redis token bucket + HTTP middleware |
| `mail` | roadmap (v0.10.0) | log · smtp · ses · sendgrid · mailgun · postmark |
| `notifications` | roadmap (v0.10.1) | mail · slack · discord · webhook · database · log channels |
| `scheduler` | roadmap (v0.10.2) | cron expressions, distributed lock via cache module |
| `featureflag` | roadmap (v0.10.3) | config · openfeature · launchdarkly · unleash · flagsmith |
| `resilience` | roadmap (v0.10.4) | retry · circuit-breaker · timeout · bulkhead primitives |
| `queue` | roadmap (v0.11) | messaging — memory · sqs · kafka · nats |
| `httpx` | roadmap (v0.12) | chi router + handler conventions |
| `observability/cloudwatch` | roadmap (v0.13) | full AWS CloudWatch driver for observability (stub today) |
| `health` | roadmap (v0.14) | `/healthz`, `/readyz`, dependency probes |

Every module follows the [driver pattern](./docs/DRIVER_PATTERN.md): top-level package, public `driver/` contract, per-implementation `drivers/<name>/` package, optional `middleware/` sub-package.

## Drivers (observability)

A **driver** is the in-process code that ships telemetry to a destination — selected once at deploy time via `OBSERVABILITY_DRIVER`. Application code never knows or imports a driver beyond an optional blank import for heavy ones.

| Driver | `OBSERVABILITY_DRIVER` | Status | Registration | Use case |
|--------|------------------------|--------|--------------|----------|
| Stdout | `stdout` | stable | auto | dev, containers (orchestrator collects stdout) |
| File   | `file`   | stable | auto | bare-metal / VM, zero-budget, Laravel-style local file |
| Stack  | `stack`  | stable | auto | fan-out: every log record to multiple sub-drivers (Laravel `stack` channel) |
| OTLP   | `otlp`   | stable | opt-in (`_ "...drivers/otlp"`) | godx-platform-observability, Datadog, New Relic, any OTLP receiver |
| CloudWatch | `cloudwatch` | stub | opt-in (`_ "...drivers/cloudwatch"`) | full impl in v0.7.0 (AWS ADOT) |

Plus **named channels** (Laravel-style per-call selection — `obs.Channel("audit").Info(...)`): see [docs/modules/observability — channels](./docs/modules/observability.md#channels-laravel-style-named-loggers). Channels can be declared in Go (`NewChannel(name, cfg)`) or purely via env vars (`OBSERVABILITY_CHANNELS=audit,billing` + per-channel env keys) using `ChannelsFromEnv()`. Each channel has its own minimum level. The `stack` driver also accepts per-sub level: `OBSERVABILITY_STACK_DRIVERS=stdout:info,file:warn`.

## Drivers (storage)

A **storage driver** is the in-process code that reads/writes objects on a specific backend — selected once at deploy time via per-disk env vars. Same opt-in convention as observability: light drivers auto-register, heavy ones require a blank import.

| Driver | `STORAGE_DISK_<NAME>_DRIVER` | Status | Registration | Use case |
|--------|------------------------------|--------|--------------|----------|
| Local  | `local`  | stable | auto | filesystem (default — `./storage/app/private` à la Laravel) |
| Memory | `memory` | stable | auto | tests, ephemeral fixtures |
| S3     | `s3`     | stable (v0.6.1) | opt-in (`_ "...drivers/s3"`) | AWS S3 — multipart streaming, presigned URLs |
| MinIO  | `minio`  | stable (v0.6.1) | opt-in (`_ "...drivers/minio"`) | MinIO / S3-compatible (R2, DO Spaces, …) — path-style by default |
| GCS    | `gcs`    | stable (v0.6.2) | opt-in (`_ "...drivers/gcs"`) | Google Cloud Storage — resumable streaming, V4 signed URLs, UBLA-safe visibility |
| Azure  | `azure`  | stable (v0.6.2) | opt-in (`_ "...drivers/azure"`) | Azure Blob Storage — streaming uploads, shared-key SAS, hierarchical list |

Each disk has its own visibility default (`public`/`private`), public URL base, and (for cloud) bucket/region/credentials. Multiple disks live side by side under one `Manager`: `mgr.Disk("avatars").Put(...)`. Full reference: [docs/modules/storage](./docs/modules/storage.md).

```go
import (
    "github.com/godx-jp/godx-platform-framework/framework"
    "github.com/godx-jp/godx-platform-framework/storage"
)

app := framework.New("svc", "1.0.0").Use(storage.Module)
_ = app.Init(ctx)

mgr, _ := storage.FromApp(app)
disk, _ := mgr.Disk("local")
_ = disk.Put(ctx, "hello.txt", []byte("world"))
body, _ := disk.Get(ctx, "hello.txt")
```

Adding a new driver: see [docs/DRIVER_PATTERN](./docs/DRIVER_PATTERN.md).

## Drivers (cache)

A **cache driver** is the in-process code that reads/writes ephemeral key/value entries on a specific backend — selected once at deploy time via per-store env vars.

| Driver | `CACHE_STORE_<NAME>_DRIVER` | Status | Registration | Use case |
|--------|------------------------------|--------|--------------|----------|
| Memory | `memory` | stable | auto | tests, single-process services, default zero-config store |
| File   | `file`   | stable | auto | bare-metal / VM, single-host persistence (Laravel `FileStore` layout under `./storage/framework/cache`) |
| Redis  | `redis`  | stable (v0.7.0) | opt-in (`_ "...cache/drivers/redis"`) | shared cache across replicas; atomic INCRBY counters; SCAN-scoped Flush by prefix |

Database-backed cache is intentionally out of scope — see [docs/modules/cache § Why no DB driver](./docs/modules/cache.md#why-no-db-driver).

## Drivers (config)

A **config driver** is a Source that contributes one tree of key/value pairs to the layered repository — selected once at deploy time via per-source env vars. Sources merge in registration order (last source wins); the auto-env source is appended after every configured source so process env always overrides files.

| Driver | `CONFIG_SOURCE_<NAME>_DRIVER` | Status | Registration | Use case |
|--------|-------------------------------|--------|--------------|----------|
| Env    | `env`    | stable | auto | Read from `os.Environ()` with optional prefix; nested via `__` |
| File   | `file`   | stable | auto | YAML / JSON / TOML on disk; optional `Watcher` polls mtime |
| Static | `static` | stable | auto | In-process map, primarily for tests and compile-time defaults |
| Remote | `remote` | roadmap | opt-in (`_ "...config/drivers/remote/<name>"`) | etcd / consul / vault — full impl in a follow-up release |

```go
import (
    "github.com/godx-jp/godx-platform-framework/config"
    "github.com/godx-jp/godx-platform-framework/framework"
)

app := framework.New("svc", "1.0.0").Use(config.Module)
_ = app.Init(ctx)

cfg, _ := config.FromApp(app)
port := cfg.GetInt("server.port", 8080)
ttl  := config.Get[time.Duration](cfg, "cache.ttl", 5*time.Minute)
```

Full reference: [docs/modules/config](./docs/modules/config.md).

## Drivers (secrets)

A **secrets driver** is a Store that fronts one secrets backend behind a uniform `Get` / `Put` / `Forget` / `List` API. Switching between dev (env) and production (vault / cloud KMS) is a configuration change, no code change.

| Driver | Visibility   | Writable     | Listable | Use case |
|--------|--------------|--------------|----------|----------|
| Env    | auto         | no           | no       | Local dev — reads `SECRETS_<KEY>` from process env |
| File   | auto         | yes (atomic) | yes      | Container / K8s — one file per secret under `SECRETS_FILE_PATH` |
| Vault  | blank-import | yes          | yes      | HashiCorp Vault KV-v2 |
| GCPSM  | blank-import | yes          | yes      | Google Cloud Secret Manager (ADC) |
| AWSSM  | blank-import | yes          | yes      | AWS Secrets Manager (`SecretBinary` per key) |

```go
import (
    _ "github.com/godx-jp/godx-platform-framework/secrets/drivers/vault"
    "github.com/godx-jp/godx-platform-framework/framework"
    "github.com/godx-jp/godx-platform-framework/secrets"
)

app := framework.New("svc", "1.0.0").Use(secrets.Module)
_ = app.Init(ctx)
mgr, _ := secrets.FromApp(app)
dbPass, _ := mgr.GetString(ctx, "db/password")
```

Full reference: [docs/modules/secrets](./docs/modules/secrets.md).

```go
import (
    "github.com/godx-jp/godx-platform-framework/cache"
    _ "github.com/godx-jp/godx-platform-framework/cache/drivers/redis"
    "github.com/godx-jp/godx-platform-framework/framework"
)

app := framework.New("svc", "1.0.0").Use(cache.Module)
_ = app.Init(ctx)

mgr, _ := cache.FromApp(app)
store := mgr.Default()

_ = store.Put(ctx, "answer", []byte("42"), 30*time.Minute)
v, ok, _ := store.Get(ctx, "answer")

n, _ := store.Increment(ctx, "visits", 1)               // atomic counter
v2, _ := store.Remember(ctx, "expensive", time.Minute,  // cache-aside
    func(ctx context.Context) ([]byte, error) {
        return compute(ctx)
    })
```

Full reference: [docs/modules/cache](./docs/modules/cache.md).

## Why this exists

The Go ecosystem has excellent low-level libraries (`log/slog`, `go.opentelemetry.io/otel`, `prometheus/client_golang`) — but every team re-implements the same wiring, the same env-var conventions, the same correlation-ID propagation. This SDK packages those conventions so every service in an organisation looks the same — and so swapping backends is a configuration change, not a refactor.

## Repository layout

```
godx-platform-framework/
├── framework/                          App backbone (Module interface, lifecycle)
├── observability/                      Logs, traces, metrics
│   ├── driver/                         Public driver contract (interface, Spec, registry)
│   ├── drivers/                        Built-in drivers — one package each
│   │   ├── stdout/ · file/ · stack/    light, auto-registered
│   │   ├── otlp/                       heavy, opt-in blank import
│   │   └── cloudwatch/                 heavy, opt-in (stub until v0.7.0)
│   └── middleware/                     Optional HTTP middleware sub-package
├── storage/                            Multi-disk file/object storage
│   ├── driver/                         Public driver contract (interface, Spec, registry)
│   └── drivers/
│       ├── local/ · memory/            light, auto-registered
│       ├── internal/s3core/            shared S3 protocol impl (used by s3 + minio)
│       ├── s3/ · minio/                heavy, opt-in blank import — stable (v0.6.1)
│       └── gcs/ · azure/               heavy, opt-in blank import — stable (v0.6.2)
├── cache/                              Laravel-style multi-store cache
│   ├── driver/                         Public driver contract (interface, Spec, registry)
│   └── drivers/
│       ├── memory/ · file/             light, auto-registered
│       └── redis/                      heavy, opt-in blank import — stable (v0.7.0)
├── config/                             Layered configuration repository
│   ├── driver/                         Public driver contract (Source, Watcher, Spec, registry)
│   └── drivers/
│       ├── env/ · file/ · static/      light, auto-registered
│       └── remote/                     heavy, opt-in blank import — roadmap
├── examples/
│   ├── minimal/                        25-line example
│   ├── http-server/                    HTTP server with traced handler + middleware
│   └── cache/                          Laravel-style cache walkthrough
├── docs/
│   ├── README.md                       docs index
│   ├── GETTING_STARTED.md
│   ├── ARCHITECTURE.md                 backbone, lifecycle, repository layout
│   ├── DRIVER_PATTERN.md               shared convention for every module
│   ├── CONFIGURATION.md
│   ├── VERSIONING.md
│   └── modules/
│       └── observability.md            per-module reference (one file per module)
└── .github/workflows/ci.yml
```

The internal layout of every future module (storage, cache, queue, ...) is identical to `observability/`. See [docs/DRIVER_PATTERN — Layout convention](./docs/DRIVER_PATTERN.md#layout-convention).

## Project status

- **Maintainer:** godx platform team
- **License:** Apache 2.0
- **Stability:** `0.x` — pin to exact tag in production until `1.0.0`
- **Go version:** 1.23+
- **Companion product:** [godx-platform-observability](https://github.com/godx-jp/godx-platform-observability) — the self-hosted observability backend referenced by the `otlp` driver

## Documentation

| Doc | Audience |
|-----|----------|
| [GETTING_STARTED](./docs/GETTING_STARTED.md) | New users — five-minute tutorial |
| [ARCHITECTURE](./docs/ARCHITECTURE.md) | Engineers — backbone, lifecycle, layout, roadmap |
| [DRIVER_PATTERN](./docs/DRIVER_PATTERN.md) | Anyone touching drivers — shared convention for every module |
| [modules/observability](./docs/modules/observability.md) | App developers using observability |
| [modules/storage](./docs/modules/storage.md) | App developers using storage |
| [modules/cache](./docs/modules/cache.md) | App developers using cache |
| [modules/config](./docs/modules/config.md) | App developers using config |
| [modules/events](./docs/modules/events.md) | App developers using events |
| [modules/hashing](./docs/modules/hashing.md) | App developers using hashing |
| [modules/encryption](./docs/modules/encryption.md) | App developers using encryption |
| [modules/pipeline](./docs/modules/pipeline.md) | App developers using pipeline |
| [modules/secrets](./docs/modules/secrets.md) | App developers using secrets |
| [CONFIGURATION](./docs/CONFIGURATION.md) | Operators — every env var |
| [VERSIONING](./docs/VERSIONING.md) | Consumers — SemVer policy |

## License

Apache 2.0 — see [LICENSE](./LICENSE).
