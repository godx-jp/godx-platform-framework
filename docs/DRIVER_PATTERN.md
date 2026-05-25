[← docs index](./README.md)

# Driver pattern

The driver pattern is the single convention every godx-platform-framework module follows for plugging in swappable backends. It applies to:

- `observability` (stdout, file, otlp, stack, cloudwatch) — shipped in v0.4
- `storage` (local, s3, gcs, azure, minio) — roadmap
- `cache` (memory, redis, memcached) — roadmap
- `queue` (in-memory, sqs, kafka, nats) — roadmap
- any future module that has a destination to swap

Read this once. The same shape repeats for every module.

## Goals

1. **Configuration-driven backend selection.** Operators change one env var and the application points at a new destination. No recompile, no application-code change.
2. **Pay-for-what-you-use dependencies.** A service that only writes to local files must not pull AWS SDKs into its binary. Heavy drivers are opt-in via blank import.
3. **Third-party extensibility.** Anyone can implement a driver in their own repo and register it — no fork of godx-platform-framework needed.
4. **Uniform discoverability.** The layout, naming, env var conventions, and registry API are identical across every module.

## Vocabulary

- **Driver** — the Go package that implements the module's contract for one backend. Built-ins live in `<module>/drivers/<name>/`; third-party drivers live anywhere.
- **Backend** — the destination service. Examples: Loki, S3, Redis, CloudWatch Logs. Selecting a driver picks a backend implicitly.
- **Spec** — the construction input passed to every driver in a module. One uniform shape so the registry can build any driver with one call.
- **Constructor** — `func(ctx, Spec) (Driver, error)`. Each driver package exports one as `New`.
- **Registry** — process-wide map of name → constructor, populated by driver `init()` functions.

## Layout convention

```
<module>/
├── doc.go                     Package overview, opening example
├── module.go                  framework.Module wiring (Init/OnShutdown)
├── config.go                  Config struct + LoadConfigFromEnv
├── provider.go                <Module>.Provider — the public handle
├── register.go                Blank imports of light, auto-registered drivers
│
├── driver/                    ← PUBLIC contract for third-party drivers
│   ├── driver.go              Driver interface + Constructor type
│   ├── spec.go                Spec struct
│   ├── registry.go            Register, Lookup, Names, New
│   └── helpers.go             Optional shared helpers (e.g. SamplerFor for OTel)
│
├── drivers/                   ← Built-in implementations, one per package
│   ├── stdout/                Light — auto-registered by register.go
│   │   └── stdout.go
│   ├── file/                  Light — auto-registered
│   │   └── file.go
│   ├── stack/                 Light meta-driver — auto-registered
│   │   └── stack.go
│   ├── otlp/                  Heavy — opt-in (consumer blank-imports)
│   │   └── otlp.go
│   └── cloudwatch/            Heavy — opt-in
│       └── cloudwatch.go
│
└── middleware/                Optional sub-package for HTTP/gRPC integration
    └── http.go
```

This is the layout shipped for the observability module. The storage, cache, and queue modules follow the same skeleton once they land — file paths just change `observability/` → `storage/`, `cache/`, `queue/`.

## Light vs heavy drivers

The single most important operational choice: which drivers ship in the default binary, which require an explicit opt-in?

| | Light drivers | Heavy drivers |
|---|---------------|---------------|
| Definition | No external dependencies beyond what the module already pulls | Pulls a large SDK (OTel exporters, AWS SDK, GCS client, etc.) |
| Registration | Auto via blank import in `<module>/register.go` | Opt-in via consumer blank import |
| When the consumer must do something | Nothing — works out of the box | `import _ "<module>/drivers/<name>"` |
| If consumer forgets and sets `<MODULE>_DRIVER=name` | N/A | Clear runtime error: `"name" not registered (heavy drivers require explicit blank import…)` |
| Examples (observability) | stdout, file, stack | otlp (~20MB OTel exporters), cloudwatch (~50MB AWS SDK once it ships) |
| Examples (storage, future) | local (filesystem) | s3 (aws-sdk-go-v2), gcs (cloud.google.com/go), azure (azure-sdk-for-go) |

This mirrors `database/sql`: every consumer blank-imports the driver they intend to use (`_ "github.com/lib/pq"`) and pays only for that.

## Lifecycle of a driver

```
T0   Driver package init() runs
     ─ <module>/drivers/stdout/stdout.go: driver.Register("stdout", New)
     ─ <module>/drivers/file/file.go:    driver.Register("file",   New)
     ─ <module>/drivers/stack/stack.go:  driver.Register("stack",  New)
     ─ if consumer blank-imports drivers/otlp:
       <module>/drivers/otlp/otlp.go:    driver.Register("otlp",   New)

T1   Application start
     ─ framework.New(...).Use(observability.Module)
     ─ app.Init(ctx)
         └── observability.Module.Init reads env, builds Config
             └── driver.New(ctx, Spec{Name: "otlp", ...})
                 ├── Lookup("otlp") → (otlpCtor, true)
                 └── otlpCtor(ctx, spec) → otlp.New(...) → instance

T2   Runtime
     ─ Provider holds the driver instance and delegates LoggerHandler /
       TracerProvider / MeterProvider calls.

T3   Application shutdown
     ─ app.Shutdown propagates to OnShutdown hooks (reverse order)
     ─ Provider.Shutdown(ctx) → driver.Shutdown(ctx)
```

## Authoring a driver

The same five-step recipe works for built-in or third-party drivers:

1. Create a package. Built-in goes under `<module>/drivers/<name>/`; third-party goes anywhere in your own module.

2. Implement the module's `driver.Driver` interface. For observability:

   ```go
   type Driver interface {
       LoggerHandler() slog.Handler
       TracerProvider() trace.TracerProvider
       MeterProvider() metric.MeterProvider
       Shutdown(ctx context.Context) error
   }
   ```

3. Export a constructor `func New(ctx context.Context, s driver.Spec) (driver.Driver, error)`.

4. Register the constructor in `init()`:

   ```go
   const Name = "yourdriver"
   func init() { driver.Register(Name, New) }
   ```

5. Consumer blank-imports your package and sets the env var:

   ```go
   import _ "example.com/godx-driver-yourdriver"
   ```
   ```bash
   OBSERVABILITY_DRIVER=yourdriver
   ```

That's all. No core change. No upstream PR.

## Anti-patterns

- **Don't construct drivers directly from application code.** Always go through the module's public Config + Provider. The registry exists so application code never imports any concrete driver beyond a blank import.
- **Don't put module-specific fields on `driver.Spec` for one third-party driver.** If your driver needs new config, accept it via env vars and read them in your constructor. Spec is reserved for fields that several drivers share (service identity, sample rate, ...).
- **Don't auto-register a driver that has heavy dependencies.** It removes the consumer's ability to keep their binary small.
- **Don't nest meta-drivers inside themselves** (e.g. `stack` inside `stack`). The framework rejects it at construction; the validation lives in each meta-driver's `New`.

## Cross-module reference

Once additional modules ship, this same convention will hold for:

- `storage.driver.Driver` — `Put(ctx, key, reader)`, `Get(ctx, key) Reader`, `Delete(ctx, key)`, ...
- `cache.driver.Driver` — `Get(ctx, key) (value, ok)`, `Set(ctx, key, value, ttl)`, ...
- `queue.driver.Driver` — `Publish(ctx, topic, msg)`, `Subscribe(ctx, topic) <-chan Message`, ...

Each module has its own `driver/` package and registry — there is no global registry shared across modules. This keeps backend names from colliding (e.g. `s3` in storage vs `s3` in queue if it ever existed).

## See also

- [ARCHITECTURE](./ARCHITECTURE.md) — framework backbone, module lifecycle
- [modules/observability.md](./modules/observability.md) — full reference for the observability module's drivers
- [CONFIGURATION](./CONFIGURATION.md) — every env var
