# Changelog

All notable changes are documented here. Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) · versioning: [SemVer](https://semver.org/).

## [Unreleased]

## [0.5.0] — 2026-05-25

Channel-system maturity release — closes the Laravel `config/logging.php` parity gap. All additions are **backward-compatible**; no consumer code change is required to upgrade from 0.4.0.

### Added — per-channel level filter

- `Config.LogLevel` on a `NewChannel(name, cfg)` config is now formally documented as the per-channel minimum level (Laravel `'level' => 'warning'`). The driver's `slog.Handler` enforces it, so records below threshold never reach the wire. Verified end-to-end by `TestChannel_PerChannelLevelFilter`.

### Added — env-driven channels (zero Go code)

- `observability.ChannelsFromEnv()` — new framework module that reads `OBSERVABILITY_CHANNELS` (comma list) and, for each name X, builds a `Config` from `OBSERVABILITY_CHANNEL_<X>_*` env vars, then registers the channel on the primary provider. Mirrors Laravel `config/logging.php` for projects that prefer 12-factor over Go code.
- `observability.LoadChannelConfigFromEnv(name)` — exported helper that returns the per-channel `Config`. Same field shape as `LoadConfigFromEnv` but with the `OBSERVABILITY_CHANNEL_<NAME>_` prefix; OTLP keys are namespaced (the global `OTEL_EXPORTER_OTLP_*` cannot be repeated per-channel).
- `observability.ChannelsEnvVar` constant (`"OBSERVABILITY_CHANNELS"`).
- Channel name normalisation: case-insensitive; hyphens/spaces convert to underscores. `audit-trail` ⇒ `OBSERVABILITY_CHANNEL_AUDIT_TRAIL_*`.
- Startup validation: reserved name (`primary`), duplicate names, wrong wiring order (`ChannelsFromEnv()` before `Module`), unknown driver, and any per-channel construction error all fail fast with a clear message.
- Wiring (one line, safe to leave in `main` permanently — no-op when `OBSERVABILITY_CHANNELS` is unset):

  ```go
  app := framework.New("svc", "1.0.0").
      Use(observability.Module).
      Use(observability.ChannelsFromEnv())
  ```

### Added — stack driver per-sub minimum level

- `OBSERVABILITY_STACK_DRIVERS` now accepts an inline `name:level` syntax per entry — Laravel "info to stdout, warn+ to file" pattern without defining named channels:

  ```bash
  OBSERVABILITY_DRIVER=stack
  OBSERVABILITY_STACK_DRIVERS=stdout:info,file:warn
  OBSERVABILITY_LOG_FILE_PATH=/var/log/app.log
  ```

  Without `:level` the sub-driver inherits the parent's `OBSERVABILITY_LOG_LEVEL` (existing behaviour). Unknown levels fail at construction with a clear error pointing at the offending sub.

### Added — `driver.ParseLogLevel` helper

- New exported helper `driver.ParseLogLevel(s string) (slog.Level, bool)` in `observability/driver` — single source of truth for the four supported levels (`debug` · `info` · `warn`/`warning` · `error`). Used internally by the observability config loader and the stack driver; available to third-party drivers via the public `driver` package.

### Tests

Nine new tests cover the additions:

- `TestStackDriver_PerSubLevelFilter` — fan-out filter respects per-sub min level.
- `TestStackDriver_PerSubLevel_RejectsUnknownLevel` — bad level fails at construction.
- `TestChannel_PerChannelLevelFilter` — `Config.LogLevel` drops records below threshold on a named channel.
- `TestChannelsFromEnv_RegistersDeclaredChannels` — full env path, per-channel level enforced.
- `TestChannelsFromEnv_NoopWhenUnset` — empty `OBSERVABILITY_CHANNELS` registers nothing.
- `TestChannelsFromEnv_RejectsPrimaryReserved` — `primary` cannot be listed.
- `TestChannelsFromEnv_RejectsDuplicate` — same name listed twice fails.
- `TestChannelsFromEnv_OrderingErrorWhenBeforeModule` — must be `.Use`d after `Module`.
- `TestLoadChannelConfigFromEnv_NormalisesName` — hyphenated channel names map to upper-case underscore segments.

### Changed (non-breaking)

- Roadmap: cloudwatch driver pushed 0.5.x → 0.6.x, storage 0.6.x → 0.7.x, cache → 0.8.x, queue → 0.9.x, httpx → 0.10.x. v0.5.x is dedicated to nailing Laravel `config/logging.php` parity.
- `observability.parseLogLevel` (internal) now delegates to `driver.ParseLogLevel` — single canonical implementation.
- `cloudwatch.ErrNotImplemented` and its package doc reference the new 0.6.0 target.

### Migration

None required. Existing wiring keeps working:

- `OBSERVABILITY_STACK_DRIVERS=stdout,file` continues to inherit `OBSERVABILITY_LOG_LEVEL` for both.
- `NewChannel(name, cfg)` is unchanged.
- `ChannelsFromEnv()` is opt-in — add it only when you want declarative channels.

## [0.4.0] — 2026-05-25

This release is a **structural breaking change** to put the framework on the same layout convention as `go-kit`, `OpenTelemetry Go`, `kratos`, and the team's own `go-common` — one package per concern at the root, public driver contract under `<module>/driver/`, built-in driver implementations split into one package each under `<module>/drivers/<name>/`, optional integration sub-packages (e.g. `<module>/middleware/`). The shape is fixed so that every future module (storage, cache, queue, httpx, ...) slots in identically.

No behaviour changes for the primary user paths (`obs.Logger()`, `obs.Tracer()`, `obs.Meter()`, env-driven driver selection, Laravel-style channels). The breaking changes are import paths and one method rename.

### Added — layout convention

- `observability/driver/` — new **public** package: `Driver` interface, `Spec`, `Constructor`, `Register / Lookup / Names / New`, plus shared helpers `SamplerFor` and `ResourceFor` for third-party drivers.
- `observability/register.go` — blank-imports the **light** built-in drivers (`stdout`, `file`, `stack`) so they auto-register when `observability` is imported, mirroring `database/sql` defaults.
- `observability/middleware/` — sub-package holding HTTP middleware. Optional dependency on `net/http`; non-HTTP services no longer transitively import it.
- `observability/doc.go` — package overview documenting the new layout.
- `docs/DRIVER_PATTERN.md` — **shared** convention for every current and future module. Describes the `<module>/driver/` + `<module>/drivers/<name>/` + `<module>/middleware/` skeleton, light vs heavy drivers, blank-import discipline, and the recipe for authoring a custom driver.
- `docs/modules/observability.md` — per-module reference (replaces and merges the old `OBSERVABILITY.md` and `DRIVERS.md`).
- `docs/README.md` — docs index.

### Changed (breaking)

- **Per-driver subpackages.** Each built-in driver now lives in its own package:

  | Old | New |
  |-----|-----|
  | `observability/drivers/stdout.go`     | `observability/drivers/stdout/stdout.go` (`package stdout`) |
  | `observability/drivers/file.go`       | `observability/drivers/file/file.go` (`package file`) |
  | `observability/drivers/otlp.go`       | `observability/drivers/otlp/otlp.go` (`package otlp`) |
  | `observability/drivers/stack.go`      | `observability/drivers/stack/stack.go` (`package stack`) |
  | `observability/drivers/cloudwatch.go` | `observability/drivers/cloudwatch/cloudwatch.go` (`package cloudwatch`) |
  | `observability/drivers/driver.go`     | `observability/driver/{driver,spec,registry,helpers}.go` (`package driver`) |

- **Driver registry instead of monolithic switch.** `drivers.New(ctx, spec)` is gone; each driver package registers itself via `init() { driver.Register(Name, New) }`. The observability package calls `driver.New(ctx, spec)` which dispatches via the registry. Missing registrations return a clear error pointing at the required blank import.

- **Light vs heavy driver split.** Light drivers (`stdout`, `file`, `stack`) stay auto-registered through `observability/register.go`. Heavy drivers (`otlp`, `cloudwatch`) now require an explicit blank import in consumer code:

  ```go
  import _ "github.com/godx-jp/godx-platform-framework/observability/drivers/otlp"
  ```

  This mirrors the `database/sql` convention and prevents non-OTLP services from pulling ~20MB of exporter dependencies. The future S3 / GCS / Redis / Kafka drivers will follow the same rule.

- **HTTP middleware moved to sub-package.** `Provider.Middleware(handler)` → `middleware.HTTP(provider)(handler)`. New import:

  ```go
  import "github.com/godx-jp/godx-platform-framework/observability/middleware"
  srv := &http.Server{Handler: middleware.HTTP(obs)(mux)}
  ```

  Constant moved: `observability.CorrelationHeader` → `middleware.CorrelationHeader`.

- **`file` driver constant rename.** `drivers.LogFileRotationNone/Daily/Size` → `file.RotationNone/Daily/Size` (in `observability/drivers/file`).

- **`cloudwatch` driver constant rename.** `drivers.ErrCloudWatchNotImplemented` → `cloudwatch.ErrNotImplemented` (in `observability/drivers/cloudwatch`).

- **Roadmap update.** CloudWatch driver moves 0.4.x → 0.5.0. Storage / cache / queue / httpx pushed back accordingly. v0.4 is dedicated to nailing the framework layout before more modules ship.

### Removed

- `docs/DRIVERS.md` — content merged into `docs/DRIVER_PATTERN.md` (cross-module) and `docs/modules/observability.md` (per-module).
- `docs/OBSERVABILITY.md` — content merged into `docs/modules/observability.md`.
- `observability/middleware.go` (root) — moved into `observability/middleware/http.go`.

### Migration

Drop-in upgrade for the common cases. Most apps need three textual changes:

```diff
 import (
     "github.com/godx-jp/godx-platform-framework/framework"
     "github.com/godx-jp/godx-platform-framework/observability"
+    "github.com/godx-jp/godx-platform-framework/observability/middleware" // HTTP only
+    _ "github.com/godx-jp/godx-platform-framework/observability/drivers/otlp" // OTLP only
 )

-srv := &http.Server{Handler: obs.Middleware(mux)}
+srv := &http.Server{Handler: middleware.HTTP(obs)(mux)}
```

Custom drivers built against the v0.3 `drivers.Driver` interface: change the package import from `.../observability/drivers` to `.../observability/driver` and rename the type references (`drivers.Driver` → `driver.Driver`, `drivers.Spec` → `driver.Spec`). Then register via `init() { driver.Register("yourname", New) }` instead of patching `drivers.New`.

## [0.3.0] — 2026-05-25

### Added — Laravel-style multi-channel logging

- `drivers.stack` — meta-driver that fans out every log record to N sub-drivers. Mirrors Laravel's `stack` log channel. Configure with `OBSERVABILITY_DRIVER=stack` and `OBSERVABILITY_STACK_DRIVERS=stdout,file` (comma-separated). Sub-drivers inherit the rest of the spec, so OTLP / file settings flow through. Nesting (`stack` inside `stack`) is rejected at construction. Traces and metrics use the first sub-driver only — duplicating spans across exporters would produce double-counted distributed traces.
- `observability.NewChannel(name, cfg)` — framework module that registers an additional named channel on top of the primary one. Order matters: `observability.Module` must be `Use`d before any `NewChannel`.
- `Provider.Channel(name) *slog.Logger` — Laravel-style per-call channel selection (`obs.Channel("audit").Info(...)`). Unknown channels fall back to the primary logger with a warn line (never panics).
- `Provider.Channels() []string` — list registered channel names for diagnostics / admin endpoints.
- `PrimaryChannel = "primary"` constant — reserved name for the default channel; cannot be overridden via `NewChannel`.
- `Config.StackDrivers []string` — new field; `OBSERVABILITY_STACK_DRIVERS` env var (comma-separated, whitespace tolerated).
- Five new test files / suites: `stack_test.go` (fan-out, nested rejection, missing list, unknown sub-driver, env parsing), `channel_test.go` (routing, reserved name, fallback, ordering error, `Channels()` lookup).

### Changed
- Roadmap: CloudWatch driver moves 0.3.0 → 0.4.0; httpx moves 0.4.0 → 0.5.0.
- Default channel is now formally named `"primary"`. Existing code that calls `obs.Logger()` is unchanged.

## [0.2.0] — 2026-05-25

This release is a **breaking rename** to clarify naming. No behaviour changes. Pin to `v0.1.0` if you need the old names; upgrade by replacing `OBS_*` → `OBSERVABILITY_*` and `Backend` → `Driver` in your code and env files.

### Added (carried from unreleased 0.1.x)
- `observability/drivers/file.go` — Laravel-style local file log driver (`none` / `daily` / `size` rotation, gzip, retention) for zero-budget / bare-metal / VM deployments where no log collector is available. Uses `gopkg.in/natefinch/lumberjack.v2`. Parent directory auto-created.

### Changed (breaking)
- **Package rename**: `observability/backends/` → `observability/drivers/`.
- **Interface rename**: `backends.Backend` → `drivers.Driver`. The Go convention (`database/sql.Driver`) and Laravel terminology both use "driver" for the swappable implementation.
- **Type rename**: `backends.Spec` → `drivers.Spec`.
- **Const rename**: `BackendStdout`/`BackendFile`/`BackendOTLP`/`BackendCloudWatch` → `DriverStdout`/`DriverFile`/`DriverOTLP`/`DriverCloudWatch`.
- **Config field rename**: `Config.Backend` → `Config.Driver`. `Config.FilePath` → `Config.LogFilePath`. `Config.FileRotate` → `Config.LogFileRotation`. `Config.File*` → `Config.LogFile*` (all file-driver fields prefixed with `LogFile`). `Config.LogGroupName` → `Config.CloudWatchLogGroup`.
- **Method rename**: `Provider.Backend()` → `Provider.Driver()`.
- **File rotation consts**: `FileRotateNone/Daily/Size` → `LogFileRotationNone/Daily/Size` (now in `drivers` package).
- **Env var rename** — all SDK env vars now use the full word `OBSERVABILITY_*` (no abbreviations); industry-standard vars (`OTEL_*`, `AWS_*`, `DEPLOYMENT_ENVIRONMENT`) are unchanged.

  | Old | New |
  |-----|-----|
  | `OBS_BACKEND` | `OBSERVABILITY_DRIVER` |
  | `OBS_LOG_LEVEL` | `OBSERVABILITY_LOG_LEVEL` |
  | `OBS_TRACE_SAMPLE` | `OBSERVABILITY_TRACE_SAMPLE_RATE` |
  | `OBS_LOG_FILE` | `OBSERVABILITY_LOG_FILE_PATH` |
  | `OBS_LOG_ROTATE` | `OBSERVABILITY_LOG_FILE_ROTATION` |
  | `OBS_LOG_MAX_SIZE_MB` | `OBSERVABILITY_LOG_FILE_MAX_SIZE_MB` |
  | `OBS_LOG_MAX_AGE_DAYS` | `OBSERVABILITY_LOG_FILE_MAX_AGE_DAYS` |
  | `OBS_LOG_MAX_BACKUPS` | `OBSERVABILITY_LOG_FILE_MAX_BACKUPS` |
  | `OBS_LOG_COMPRESS` | `OBSERVABILITY_LOG_FILE_COMPRESS` |
  | `OBS_LOG_GROUP` | `OBSERVABILITY_CLOUDWATCH_LOG_GROUP` |

- **Doc rename**: `docs/BACKENDS.md` → `docs/DRIVERS.md`, with a new "Vocabulary" section explaining driver-vs-backend.
- Roadmap: CloudWatch driver moves from 0.2.0 → 0.3.0; 0.3.0 now lands AWS ADOT + configurable correlation header; httpx targeted for 0.2.x → 0.3.x.

## [0.1.0] — 2026-05-25

### Added
- `framework/` — module-based application backbone with graceful shutdown.
- `observability/` — drop-in observability module covering structured logging, OpenTelemetry traces, and metrics.
- `observability/backends/` — pluggable backend drivers:
  - `stdout` — JSON to stdout, in-process tracer (dev / tests).
  - `otlp` — OTLP gRPC / HTTP exporter (Loki/Tempo/Prometheus, Datadog, New Relic, any OTLP-compatible).
  - `cloudwatch` — placeholder stub for AWS Distro for OpenTelemetry (full implementation tracked for 0.2.0).
- `observability` HTTP middleware: trace context, correlation ID, request log.
- `examples/minimal`, `examples/http-server`.
- Configuration via environment variables (12-factor) with sensible defaults.
- Apache 2.0 license.
