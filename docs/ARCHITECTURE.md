[← docs index](./README.md)

# Architecture

godx-platform-framework is intentionally small — a backbone for modules, not a framework that owns your `main`. Every concern (observability and storage today; cache, queue, http tomorrow) is its own module and follows the same layout.

## Design principles

1. **Composition over configuration.** No DI graph reflection, no magic. `app.Use(module)` and `module.Init(ctx, app)` are the only contracts.
2. **Modules are pluggable, the backbone is not.** Adding a feature means adding a module; you never patch the core.
3. **One module per concern at the repository root.** Mirrors `go-kit`, `kratos`, `go-common`. No god-package, no top-level `pkg/` indirection.
4. **Driver pattern for destinations.** Every module that talks to a backend exposes a swappable driver — see [DRIVER_PATTERN](./DRIVER_PATTERN.md).
5. **Pay-for-what-you-use dependencies.** Light drivers are auto-registered; heavy drivers require a blank import. A service that only uses stdout never pulls AWS or OTLP into the binary.
6. **Stdlib-first.** Where the stdlib is sufficient (`log/slog`, `net/http`), we wrap it; we do not replace it.
7. **No abbreviations in env vars.** Every variable starts with the spelled-out namespace (`OBSERVABILITY_*`) unless it is a documented industry standard (`OTEL_*`, `AWS_*`, `DEPLOYMENT_ENVIRONMENT`).

## Repository layout

```
godx-platform-framework/
├── go.mod                              github.com/godx-jp/godx-platform-framework
├── README · CHANGELOG · VERSION · LICENSE · Makefile · .golangci.yml
│
├── framework/                          Core backbone: App, Module, lifecycle
│
├── observability/                      Module — logs / traces / metrics
│   ├── doc.go · module.go · config.go · provider.go · channel.go · context.go
│   ├── register.go                     Blank-imports light drivers
│   ├── driver/                         Public driver contract (interface, Spec, registry)
│   ├── drivers/                        Built-in drivers, one package each
│   │   ├── stdout/ · file/ · stack/    (light — auto-registered)
│   │   ├── otlp/                       (heavy — opt-in blank import)
│   │   └── cloudwatch/                 (heavy — opt-in; stub until v0.7.0)
│   └── middleware/                     Optional HTTP middleware sub-package
│
├── storage/                            Module — Laravel-style multi-disk file/object storage
│   ├── doc.go · module.go · config.go · disk.go · manager.go · context.go
│   ├── register.go                     Blank-imports light drivers
│   ├── driver/                         Public driver contract (interface, Spec, registry, Visibility)
│   └── drivers/                        Built-in drivers, one package each
│       ├── local/ · memory/            (light — auto-registered)
│       ├── internal/s3core/            shared S3 protocol impl (used by s3 + minio)
│       ├── s3/ · minio/                (heavy — opt-in; stable v0.6.1)
│       └── gcs/ · azure/               (heavy — opt-in; stable v0.6.2)
├── cache/                              Laravel-style multi-store cache (v0.7.0)
│   ├── driver/                         Public driver contract
│   └── drivers/
│       ├── memory/ · file/             (light — auto-registered)
│       └── redis/                      (heavy — opt-in; go-redis/v9)
│
├── config/                             Layered configuration repository (v0.8.0)
│   ├── driver/                         Public driver contract (Source, Watcher, Spec, registry)
│   └── drivers/
│       ├── env/ · file/ · static/      (light — auto-registered)
│       └── remote/                     (heavy — opt-in; roadmap — etcd / consul / vault)
│
├── events/                             In-process bus (v0.8.1)
│   ├── bus.go · async.go               sync bus + worker-pool wrapper
│   └── matcher.go                      wildcard pattern matching
├── hashing/ encryption/ pipeline/ secrets/              Future (v0.8.x security & utility primitives)
├── validation/ httpclient/ ratelimit/                   Future (v0.9.x)
├── mail/ notifications/ scheduler/ featureflag/ resilience/  Future (v0.10.x)
├── queue/                              Future (v0.11)
├── httpx/                              Future (v0.12)
├── health/                             Future (v0.14)
│
├── examples/                           Runnable programs — minimal, http-server, …
└── docs/                               This directory
    ├── README.md · GETTING_STARTED.md · ARCHITECTURE.md
    ├── DRIVER_PATTERN.md               Shared convention for every module
    ├── CONFIGURATION.md · VERSIONING.md
    └── modules/observability.md        Per-module reference (one file per module)
```

The internal layout of every future module is identical to `observability/` — see [DRIVER_PATTERN — Layout convention](./DRIVER_PATTERN.md#layout-convention).

## Backbone (the `framework` package)

```
framework.New(name, version)        →  *App
  .Use(module)                      →  *App (chainable; registration order preserved)
  .Init(ctx)                        →  call Module.Init on each in order
  .Run(ctx)                         →  block until ctx canceled or signal
  .Shutdown(ctx)                    →  run OnShutdown hooks in reverse order

Module interface:
  Name()                            string
  Init(ctx, app)                    error    // may app.Store(key, val), app.OnShutdown(fn)
```

A `Module` is the only contract the framework defines. No DI graph, no reflection, no service locator.

## Lifecycle

```
framework.New(name, version)
   │
   ▼
app.Use(observability.Module)            ┐
app.Use(...)                             │  registration (order matters)
                                         ┘
   │
   ▼
app.Init(ctx)                            ┐
   ├─ Module 1: Init(ctx, app)           │  init in order
   ├─ Module 2: Init(ctx, app)           │  each may app.Store(), app.OnShutdown()
   └─ Module N: Init(ctx, app)           ┘
   │
   ▼
app.Run(ctx)
   │
   ├─ blocks until ctx canceled OR SIGINT/SIGTERM
   │
   ▼
app.Shutdown(ctx)
   ├─ hook N
   ├─ hook ... ┐  reverse order, errors joined
   └─ hook 1   ┘
```

## Layered view

```
┌──────────────────────────────────────────────────────────────────┐
│  Your service                                                    │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │  framework.App                                             │  │
│  │  ┌──────────────────────────────────────────────────────┐  │  │
│  │  │  observability.Module                                │  │  │
│  │  │  ┌────────────────────────────────────────────────┐  │  │  │
│  │  │  │  observability.Provider                        │  │  │  │
│  │  │  │   - slog.Logger (with context handler)         │  │  │  │
│  │  │  │   - trace.Tracer (OTel)                        │  │  │  │
│  │  │  │   - metric.Meter (OTel)                        │  │  │  │
│  │  │  │   - Channel("audit") *slog.Logger              │  │  │  │
│  │  │  └────────────┬───────────────────────────────────┘  │  │  │
│  │  │               │ driver.Driver (resolved by registry)  │  │  │
│  │  │               ▼                                       │  │  │
│  │  │  ┌────────────────────────────────────────────────┐  │  │  │
│  │  │  │  drivers/stdout · file · stack · otlp · ...    │  │  │  │
│  │  │  └────────────────────────────────────────────────┘  │  │  │
│  │  └──────────────────────────────────────────────────────┘  │  │
│  │                                                             │  │
│  │  ┌──────────────────────────────────────────────────────┐  │  │
│  │  │  middleware (observability/middleware) — optional    │  │  │
│  │  │  middleware.HTTP(obs)(handler) — span + correlation  │  │  │
│  │  └──────────────────────────────────────────────────────┘  │  │
│  │                                                             │  │
│  │  (future) storage · cache · queue · httpx · eventbus       │  │
│  └─────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────┘
```

## Driver pattern (uniform across modules)

```
              ┌────────────────────┐
              │ application code   │
              │ obs.Logger().Info  │
              │ obs.Tracer().Start │
              └─────────┬──────────┘
                        │ stable API (slog, OTel)
                        ▼
              ┌────────────────────┐
              │ <module>.Provider  │
              └─────────┬──────────┘
                        │ driver.Driver interface
                        ▼
   ┌──────────┬──────────┬──────────┬──────────┬──────────────┐
   │ stdout   │ file     │ stack    │ otlp     │ cloudwatch   │
   │ (light,  │ (light,  │ (light,  │ (heavy,  │ (heavy,      │
   │  auto)   │  auto)   │  auto)   │  opt-in) │  opt-in)     │
   └──────────┴──────────┴──────────┴──────────┴──────────────┘

  selected by:  OBSERVABILITY_DRIVER=stdout|file|stack|otlp|cloudwatch
  opt-in:       import _ ".../observability/drivers/otlp"
```

The application **never** imports an exporter or a vendor SDK. Swapping the driver is a deployment configuration change, not a recompile.

The same pattern repeats for every future module — see [DRIVER_PATTERN](./DRIVER_PATTERN.md).

## Why a separate "framework" repo

- **Reusability across teams.** Consumed by multiple products (tiximax is the first); each pulls a pinned SemVer tag.
- **Independent release cadence.** The framework can iterate (e.g. ship 0.5 with storage) without touching consumer repos.
- **Pluggable testing.** Drivers and modules are interchangeable, which keeps unit tests fast and hermetic (use `stdout` everywhere).

## What this is NOT

- Not a full DI framework. No reflective wiring, no service graphs. If you need that, use [uber-go/fx](https://github.com/uber-go/fx).
- Not an HTTP framework. The `observability/middleware` sub-package is `net/http`-compatible; bring your own router (chi, echo, gin, mux, ...).
- Not a config framework with schema validation. v0.8.0 ships a layered repository with typed accessors; schema validation is the `validation` module's job (v0.9.0).

## Roadmap

| Version | Theme | Highlights |
|---------|-------|------------|
| 0.1.x | initial scaffold | first cut of framework + observability (deprecated naming) |
| 0.2.x | naming cleanup | env-var rename to full `OBSERVABILITY_*`; `Backend` → `Driver` |
| 0.3.x | multi-channel | `stack` driver (Laravel fan-out), named channels (`obs.Channel("audit")`) |
| 0.4.x | layout standardisation | per-driver subpackages, `<module>/driver` registry, `<module>/middleware` sub-package, opt-in heavy drivers |
| 0.5.x | channel maturity | per-channel level filter, env-driven channels (`ChannelsFromEnv()`), per-sub stack level (`stdout:info,file:warn`) — Laravel `config/logging.php` parity |
| 0.6.x | **`storage` module — complete** | Laravel `Storage` parity. v0.6.0: local + memory · v0.6.1: s3 + minio (shared `internal/s3core`) · **v0.6.2: gcs + azure**. All six drivers stable; module closes at 0.6.2 |
| 0.7.x | **`cache` module — shipped** | Laravel `Cache` parity. **v0.7.0: memory + file + redis** (Manager + Store + JSON helpers + atomic counters + per-key locking for file). DB-backed cache intentionally out of scope |
| **0.8.x** | **Foundation + security/utility primitives — Laravel-parity reshuffle** | **0.8.0: `config` (env/file/static/remote)** · **0.8.1: `events` (sync + async wrapper, wildcards)** · 0.8.2: `hashing` (bcrypt/argon2id/scrypt) · 0.8.3: `encryption` (aesgcm/chacha20poly1305) · 0.8.4: `pipeline` · 0.8.5: `secrets` (env/file/vault/gcpsm/awssm) |
| 0.9.x | Validation + HTTP client + rate limit | 0.9.0: `validation` (struct-tag DSL + i18n) · 0.9.1: `httpclient` (stdlib + resilient + OTel) · 0.9.2: `ratelimit` (memory + redis token bucket + middleware) |
| 0.10.x | Mail + notifications + scheduling + feature flags + resilience | 0.10.0: `mail` (log/smtp/ses/sendgrid/mailgun/postmark) · 0.10.1: `notifications` · 0.10.2: `scheduler` · 0.10.3: `featureflag` · 0.10.4: `resilience` |
| 0.11.x | `queue` module | drivers: memory · sqs · kafka · nats (lifecycle hooks via events module) |
| 0.12.x | `httpx` module | chi router + handler conventions; integrates pipeline / validation / ratelimit middleware |
| 0.13.x | `cloudwatch` driver | full AWS ADOT impl (replaces the stub) |
| 0.14.x | `health` module | `/healthz`, `/readyz`, dependency probes |
| 1.0.0 | API freeze | SemVer guarantees for `1.x` |
