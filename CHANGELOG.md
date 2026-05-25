# Changelog

All notable changes are documented here. Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) · versioning: [SemVer](https://semver.org/).

## [Unreleased]

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
