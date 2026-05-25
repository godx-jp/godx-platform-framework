# Changelog

All notable changes are documented here. Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) · versioning: [SemVer](https://semver.org/).

## [Unreleased]

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
