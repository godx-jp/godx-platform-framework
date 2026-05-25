# Changelog

All notable changes are documented here. Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) · versioning: [SemVer](https://semver.org/).

## [Unreleased]

### Added
- `observability/backends/file.go` — Laravel-style local file log driver.
  - `OBS_BACKEND=file` selects it.
  - `OBS_LOG_ROTATE`: `none` (Laravel `single` channel) · `daily` (Laravel `daily`) · `size` (rotate by `OBS_LOG_MAX_SIZE_MB`).
  - `OBS_LOG_MAX_AGE_DAYS`, `OBS_LOG_MAX_BACKUPS`, `OBS_LOG_COMPRESS` for retention.
  - Parent directory auto-created. Rotation uses `gopkg.in/natefinch/lumberjack.v2`.
  - Targets zero-budget / bare-metal / VM deployments where no log collector is available.
- `LoadConfigFromEnv` reads the new `OBS_LOG_*` env vars.
- New tests under `observability/file_test.go`.

### Changed
- Roadmap: CloudWatch driver moves from 0.2.0 → 0.3.0; 0.2.0 now plans `httpx` + stack/multi-backend log driver.

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
