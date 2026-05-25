# godx-platform-framework

> **Opinionated Go SDK by godx** — modular, OpenTelemetry-native, backend-agnostic.
> Write once, swap backends (godx-platform-observability ↔ AWS CloudWatch ↔ Datadog ↔ …) by changing one env var.

[![Version](https://img.shields.io/badge/version-0.4.0-blue.svg)](./CHANGELOG.md)
[![License](https://img.shields.io/badge/license-Apache_2.0-green.svg)](./LICENSE)
[![Maintainer](https://img.shields.io/badge/by-godx-black.svg)](#)
[![Go](https://img.shields.io/badge/go-1.23+-00ADD8.svg)](https://go.dev)

## Who is this for

Any Go team that wants production-grade infrastructure without writing it from scratch — and without being locked into one vendor. v0.4 ships the observability module; storage, cache, queue, and httpx are on the roadmap and follow the same conventions.

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
| `storage` | roadmap (v0.6) | object storage — local · s3 · gcs · azure · minio |
| `cache` | roadmap (v0.7) | caching — memory · redis · memcached |
| `queue` | roadmap (v0.8) | messaging — in-memory · sqs · kafka · nats |
| `httpx` | roadmap (v0.9) | chi router + handler conventions |

Every module follows the [driver pattern](./docs/DRIVER_PATTERN.md): top-level package, public `driver/` contract, per-implementation `drivers/<name>/` package, optional `middleware/` sub-package.

## Drivers (observability)

A **driver** is the in-process code that ships telemetry to a destination — selected once at deploy time via `OBSERVABILITY_DRIVER`. Application code never knows or imports a driver beyond an optional blank import for heavy ones.

| Driver | `OBSERVABILITY_DRIVER` | Status | Registration | Use case |
|--------|------------------------|--------|--------------|----------|
| Stdout | `stdout` | stable | auto | dev, containers (orchestrator collects stdout) |
| File   | `file`   | stable | auto | bare-metal / VM, zero-budget, Laravel-style local file |
| Stack  | `stack`  | stable | auto | fan-out: every log record to multiple sub-drivers (Laravel `stack` channel) |
| OTLP   | `otlp`   | stable | opt-in (`_ "...drivers/otlp"`) | godx-platform-observability, Datadog, New Relic, any OTLP receiver |
| CloudWatch | `cloudwatch` | stub | opt-in (`_ "...drivers/cloudwatch"`) | full impl in 0.5.0 (AWS ADOT) |

Plus **named channels** (Laravel-style per-call selection — `obs.Channel("audit").Info(...)`): see [docs/modules/observability — channels](./docs/modules/observability.md#channels-laravel-style-named-loggers).

Adding a new driver: see [docs/DRIVER_PATTERN](./docs/DRIVER_PATTERN.md).

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
│   │   └── cloudwatch/                 heavy, opt-in (stub until v0.5.0)
│   └── middleware/                     Optional HTTP middleware sub-package
├── examples/
│   ├── minimal/                        25-line example
│   └── http-server/                    HTTP server with traced handler + middleware
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
| [CONFIGURATION](./docs/CONFIGURATION.md) | Operators — every env var |
| [VERSIONING](./docs/VERSIONING.md) | Consumers — SemVer policy |

## License

Apache 2.0 — see [LICENSE](./LICENSE).
