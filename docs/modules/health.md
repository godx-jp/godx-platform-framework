# Health

> Kubernetes-style `/healthz` (liveness) and `/readyz` (readiness) endpoints
> backed by a registry of dependency probes. Liveness reports process-up;
> readiness runs every registered probe and fails closed when any probe errors.

## Concepts

A `Registry` holds named readiness probes. A `Probe` is a `func(ctx) error` — returning `nil` means healthy. Liveness (`/healthz`) always returns `200` while the process is running; readiness (`/readyz`) runs every registered probe and returns `503` if any of them fail. The module publishes a `Registry` into the framework Store; HTTP handlers are mounted separately so the module stays transport-agnostic.

```
Registry ── named Probes (func(ctx) error)
   ├─ Healthz   → 200 {"status":"ok"}        (liveness, no probes)
   └─ Readyz    → runs every probe
         ├─ all pass → 200 {"status":"ready"}
         └─ any fail → 503 {"status":"not_ready","failures":{...}}
```

## Quick start

```go
package main

import (
    "context"
    "net/http"

    "github.com/godx-jp/godx-platform-framework/framework"
    "github.com/godx-jp/godx-platform-framework/health"
)

func main() {
    ctx := context.Background()
    app := framework.New("svc", "1.0.0").Use(health.Module)
    if err := app.Init(ctx); err != nil { panic(err) }

    reg, _ := health.FromApp(app)
    reg.RegisterProbe("database", func(ctx context.Context) error {
        return db.PingContext(ctx)
    })

    http.ListenAndServe(":8080", health.Handler(reg, health.Options{}))
}
```

The module has no environment configuration — it publishes an empty `Registry` and you attach probes at startup.

## Programmatic config

Pass a pre-built `Registry` (for example one shared with another subsystem) instead of letting the module create an empty one:

```go
reg := health.NewRegistry()
reg.RegisterProbe("cache", cachePing)
app := framework.New("svc", "1.0.0").Use(health.ModuleWithRegistry(reg))
```

`ModuleWithRegistry(nil)` fails at init with `health: nil Registry`.

## Endpoints

| Path | Constant | Purpose |
|---|---|---|
| `/healthz` | `health.PathHealthz` | Liveness — `200 {"status":"ok"}` while the process is up. Runs no probes |
| `/readyz` | `health.PathReadyz` | Readiness — runs every registered probe. `200 {"status":"ready"}` when all pass; `503 {"status":"not_ready","failures":{<probe>:<error>}}` otherwise |

## API

| Symbol | Signature | Notes |
|---|---|---|
| `health.NewRegistry()` | `*Registry` | Empty registry |
| `Registry.RegisterProbe(name, p)` | `(string, Probe)` | Adds a readiness probe. Empty name or nil probe is ignored. **Re-registering the same name overwrites** the prior probe |
| `Registry.Probes()` | `[]string` | Sorted names of registered probes |
| `Registry.CheckReady(ctx)` | `(context.Context) map[string]error` | Runs every probe, returns a map of failing probe name → error (empty map = ready) |
| `health.Handler(reg, opts)` | `(*Registry, Options) http.Handler` | A stdlib `*ServeMux` serving both paths |
| `health.Mount(mux, reg, opts)` | mounts both paths onto any mux exposing `HandleFunc(pattern, http.HandlerFunc)` (e.g. a chi router) |
| `health.Healthz` | `http.HandlerFunc` | Liveness handler, usable standalone |
| `health.Readyz(reg, opts)` | `(*Registry, Options) http.HandlerFunc` | Readiness handler, usable standalone |

`Probe` type: `type Probe func(ctx context.Context) error`.

## Options

```go
health.Options{
    ProbeTimeout: 3 * time.Second, // bounds each readiness check; default 5s
}
```

`ProbeTimeout` wraps the request context with a deadline before `/readyz` runs the probes, so a hung dependency cannot stall the readiness endpoint. A zero or negative value falls back to `5s`.

## chi integration

`Handler` returns a stdlib `*ServeMux`. To mount the same paths on an existing router, use `Mount`:

```go
health.Mount(r, reg, health.Options{ProbeTimeout: 3 * time.Second})
```

See [examples/health/main.go](../../examples/health/main.go).

## Error model

Probe errors are never returned to the caller of `/readyz` as a Go error — they are collected by `Registry.CheckReady` into a `map[string]error` and rendered into the JSON response body keyed by probe name:

```json
{"status":"not_ready","failures":{"database":"dial tcp: connection refused"}}
```

A probe that returns `nil` is healthy. Probes run sequentially under the shared `ProbeTimeout` context; a timeout surfaces as that probe's error.

## Context propagation

`health.ContextWithRegistry(ctx, reg)` attaches a `Registry` to a context; `health.FromContext(ctx)` retrieves it (`ok == false` when absent). `health.FromApp(app)` is the canonical way to retrieve the `Registry` published by `health.Module`.

## Lifecycle

The module registers no `OnShutdown` callback — the `Registry` holds no resources of its own; the probes you register own (and should close) their own dependencies.
