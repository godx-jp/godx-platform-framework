# godx-platform-framework

> **Opinionated Go SDK by godx** — modular, OpenTelemetry-native, backend-agnostic.
> Write once, swap backends (godx-platform-observability ↔ AWS CloudWatch ↔ Datadog ↔ …) by changing one env var.

[![Version](https://img.shields.io/badge/version-0.3.0-blue.svg)](./CHANGELOG.md)
[![License](https://img.shields.io/badge/license-Apache_2.0-green.svg)](./LICENSE)
[![Maintainer](https://img.shields.io/badge/by-godx-black.svg)](#)
[![Go](https://img.shields.io/badge/go-1.23+-00ADD8.svg)](https://go.dev)

## Who is this for

Any Go team that wants production-grade observability without writing it from scratch — and without being locked into one vendor.

| You have… | This gives you… |
|-----------|-----------------|
| A new service | A `framework.New(...).Use(observability.Module).Run(ctx)` boilerplate |
| Many services across teams | A shared SDK with the same log format, the same trace IDs, the same metric names |
| **Zero budget / bare metal** | **Log to a local file (Laravel-style `single` / `daily` rotation)** |
| Self-hosted observability | Point at `godx-platform-observability` via OTLP |
| AWS-only budget | Point at CloudWatch (one env var change) |
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

# Self-hosted — push to godx-platform-observability
OBSERVABILITY_DRIVER=otlp \
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4317 \
go run .

# Datadog — same OTLP path, just different endpoint
OBSERVABILITY_DRIVER=otlp \
OTEL_EXPORTER_OTLP_ENDPOINT=http://datadog-agent:4317 \
go run .
```

## Modules in v0.1

| Module | Status | Purpose |
|--------|--------|---------|
| `framework` | ✅ stable | App backbone — module registration, lifecycle, graceful shutdown |
| `observability` | ✅ stable | Logs (slog JSON) + traces (OTel) + metrics (Prometheus + OTel) |

Future modules (roadmap, not in v0.1): `httpx`, `dbx`, `cachex`, `queuex`, `eventbus`, `config`.

## Drivers (observability)

A **driver** is the in-process code that ships telemetry to a destination ("backend") — selected once at deploy time via `OBSERVABILITY_DRIVER`. Application code never knows or imports a driver.

| Driver | `OBSERVABILITY_DRIVER` | Status | Use case |
|--------|------------------------|--------|----------|
| Stdout | `stdout` | ✅ Working | dev, containers (orchestrator collects stdout) |
| File   | `file`   | ✅ Working | bare-metal / VM, zero-budget, Laravel-style local file (`none` / `daily` / `size` rotation, gzip, retention) |
| OTLP   | `otlp`   | ✅ Working | godx-platform-observability, Datadog, New Relic, any OTLP receiver |
| Stack  | `stack`  | ✅ Working | fan-out: every log record to multiple sub-drivers (Laravel `stack` channel) |
| CloudWatch | `cloudwatch` | 🚧 Stub | full impl in 0.4.0 (AWS ADOT) |

Plus **named channels** (Laravel-style per-call selection — `obs.Channel("audit").Info(...)`): see [docs/OBSERVABILITY.md#channels](./docs/OBSERVABILITY.md#channels).

Adding a new driver: see [docs/DRIVERS.md](./docs/DRIVERS.md).

## Why this exists

The Go ecosystem has excellent low-level libraries (`log/slog`, `go.opentelemetry.io/otel`, `prometheus/client_golang`) — but every team re-implements the same wiring, the same env-var conventions, the same correlation-ID propagation. This SDK packages those conventions so every service in an organisation looks the same — and so swapping backends is a configuration change, not a refactor.

## Repository layout

```
godx-platform-framework/
├── framework/                    # App backbone (Module interface, lifecycle)
├── observability/                # Logging, tracing, metrics
│   └── drivers/                  # stdout · file · otlp · cloudwatch
├── examples/
│   ├── minimal/                  # 25-line example
│   └── http-server/              # HTTP server with traced handler
├── docs/
│   ├── GETTING_STARTED.md
│   ├── ARCHITECTURE.md
│   ├── OBSERVABILITY.md
│   ├── DRIVERS.md
│   ├── CONFIGURATION.md
│   └── VERSIONING.md
└── .github/workflows/ci.yml
```

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
| [ARCHITECTURE](./docs/ARCHITECTURE.md) | Engineers — modular design and lifecycle |
| [OBSERVABILITY](./docs/OBSERVABILITY.md) | App developers — using the SDK |
| [DRIVERS](./docs/DRIVERS.md) | Platform engineers — drivers and how to add one |
| [CONFIGURATION](./docs/CONFIGURATION.md) | Operators — every env var |
| [VERSIONING](./docs/VERSIONING.md) | Consumers — SemVer policy |

## License

Apache 2.0 — see [LICENSE](./LICENSE).
