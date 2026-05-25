# Architecture

godx-platform-framework is intentionally small — a backbone for modules, not a framework that owns your `main`.

## Design principles

1. **Composition over configuration.** No magic, no DI graph reflection. `app.Use(module)` and `module.Init(ctx, app)` are the only contracts.
2. **Modules are pluggable, the backbone is not.** Adding a feature means adding a module; you never patch the core.
3. **Wire-format-first.** Telemetry is shipped via OpenTelemetry (OTLP) wherever possible — never a vendor SDK in user code.
4. **Driver pattern for backends.** Selecting Loki/Tempo vs CloudWatch vs Datadog is a config decision, not a code change.
5. **Stdlib-first.** Where the stdlib is sufficient (`log/slog`, `net/http`), we wrap it; we do not replace it.

## Layered model

```
┌──────────────────────────────────────────────────────────────┐
│  Your service                                                │
│  ┌────────────────────────────────────────────────────────┐  │
│  │  framework.App                                         │  │
│  │  ┌──────────────────────────────────────────────────┐  │  │
│  │  │  observability.Module                            │  │  │
│  │  │  ┌────────────────────────────────────────────┐  │  │  │
│  │  │  │  observability.Provider                    │  │  │  │
│  │  │  │  - slog.Logger (with context handler)      │  │  │  │
│  │  │  │  - trace.Tracer                            │  │  │  │
│  │  │  │  - metric.Meter                            │  │  │  │
│  │  │  │  - http.Handler middleware                 │  │  │  │
│  │  │  └────────────────────────────────────────────┘  │  │  │
│  │  │                  │ uses                           │  │  │
│  │  │                  ▼                                │  │  │
│  │  │  ┌────────────────────────────────────────────┐  │  │  │
│  │  │  │  backends.Backend (driver)                 │  │  │  │
│  │  │  │  stdout · otlp · cloudwatch (stub)         │  │  │  │
│  │  │  └────────────────────────────────────────────┘  │  │  │
│  │  └──────────────────────────────────────────────────┘  │  │
│  │  (future) httpx · dbx · cachex · queuex · eventbus     │  │
│  └────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────┘
```

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

## Driver pattern (observability)

```
              ┌────────────────────┐
              │ application code   │
              │ obs.Logger().Info  │
              │ obs.Tracer().Start │
              └─────────┬──────────┘
                        │ stable API (slog, OTel)
                        ▼
              ┌────────────────────┐
              │ observability.     │
              │   Provider         │
              └─────────┬──────────┘
                        │ Backend interface
                        ▼
        ┌──────────┬────────────┬───────────────┐
        │ stdout   │ otlp       │ cloudwatch    │
        │ (dev)    │ (LGTM/etc) │ (AWS, 0.2.0)  │
        └──────────┴────────────┴───────────────┘

   selected by:  OBS_BACKEND=stdout|otlp|cloudwatch
```

The application **never** imports an exporter or a vendor SDK. Swapping backends is a deployment configuration change, not a recompile.

## Why a separate "framework" repo

- **Reusability across teams.** `godx-platform-framework` is consumed by multiple products (tiximax is just the first); each pulls a pinned SemVer tag.
- **Independent release cadence.** The framework can iterate (e.g. ship 0.2.0 with the CloudWatch driver) without touching consumer repos.
- **Pluggable testing.** Backends and modules are interchangeable, which keeps unit tests fast and hermetic (use `stdout` everywhere).

## What this is NOT

- Not a full DI framework. No reflective wiring, no service graphs. If you need that, use [uber-go/fx](https://github.com/uber-go/fx) instead.
- Not an HTTP framework. The observability middleware is `net/http`-compatible; bring your own router (chi, echo, gin, mux, etc.).
- Not a config framework (yet). v0.1 reads from env only; richer config is on the roadmap.

## Roadmap

| Version | Modules added |
|---------|---------------|
| 0.1.x | `framework`, `observability` (stdout/otlp + cloudwatch stub) |
| 0.2.x | `observability` cloudwatch driver (AWS ADOT), `httpx` (chi router + handlers) |
| 0.3.x | `dbx` (sqlc + outbox), `cachex` |
| 0.4.x | `queuex`, `eventbus` |
| 1.0.0 | API freeze; semver guarantees for `1.x` |
