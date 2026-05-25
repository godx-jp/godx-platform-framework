# Observability & Error Reporting Guide

How a service built on this framework emits **logs, traces, metrics**, and **early error
warnings** — and the guarantee that a stalled telemetry sink never takes the service down.

Design rationale and the full standards mapping live in
[RFC 0001](proposals/0001-error-observability-and-early-warning.md). Per-package reference:
[observability](modules/observability.md) · [httpx](modules/httpx.md) ·
[resilience](modules/resilience.md).

## The four signals

| Signal | API | Standard |
|--------|-----|----------|
| Logs | `obs.Logger()` (`*slog.Logger`), context-aware | structured JSON, severity-mapped |
| Traces | `obs.Tracer()` (OTel) | W3C Trace Context, OTel HTTP spans |
| Metrics | RED via the HTTP middleware | OTel HTTP server semconv, RED method |
| Errors / alerts | `observability.ErrorReporter` | central sink → log + metric + notify |

## Quick start — an instrumented HTTP service

```go
obs := observability.FromApp(app)

// One correct entry point: request-id + tracing + RED metrics + severity logging + recover.
r := middleware.InstrumentedRouter(obs)
r.Get("/users/{id}", httpx.Serve(getUser)) // route template => low-cardinality labels

// Central error reporting + early alerting.
rep := observability.NewReporter(obs, observability.ReporterOptions{
    Notifier:       slackNotifier,        // optional; adapt the notifications module
    NotifyMinLevel: slog.LevelError,
})
app.OnShutdown(rep.Shutdown)

// Bridge every layer's errors into the reporter (no hard coupling — plain func adapters):
httpx.SetErrorObserver(observability.HTTPErrorObserver(rep))                 // request/response
// scheduler.Options{OnRun: observability.JobErrorHook(rep, "scheduler")}
// events.AsyncOptions{OnError: observability.ErrorHook(rep, "events")}
// outbox poller Options{OnError: observability.ErrorHook(rep, "outbox")}
```

## What you get on every request

- A server span named `GET /users/{id}` (route **template**, not `/users/42`) with
  `http.route`, `http.response.status_code`, and `error.type` + `codes.Error` on 5xx.
- RED metrics: `http.server.request.duration` (histogram) and `http.server.active_requests`.
  Error rate is **derived** from the histogram filtered by status — no double-counting.
- One log line at status-mapped severity: **5xx → ERROR, 4xx → WARN, else INFO**, carrying
  `http.route`, `trace_id`, and `correlation_id`.
- Any error returned by a handler is recorded on the span and reported (see below).

## Early warning (alerting)

`ErrorReporter` fans each report out to (1) the log at the right severity, (2) the
`errors.reported` counter keyed by `source`+`error.type`, and (3) an optional, **rate-limited**
notifier. A circuit breaker opening (`resilience.OnStateChange`, logged at WARN by the
httpclient driver) is a leading indicator worth alerting on.

Example alert (OTel → Prometheus): 5xx ratio over 5 minutes —

```promql
sum(rate(http_server_request_duration_seconds_count{http_response_status_code=~"5.."}[5m]))
  / sum(rate(http_server_request_duration_seconds_count[5m])) > 0.05
```

## A stalled sink must never block the service

Logging and alerting happen on request goroutines, so a slow sink (full disk, blocked stdout
pipe, hung Slack endpoint) could cause head-of-line blocking. The framework follows the
industry rule **"telemetry must never block the application"** (Logback `AsyncAppender`
`neverBlock`, Log4j2 `AsyncLogger`, Zap `BufferedWriteSyncer`):

| Path | Mitigation | Default |
|------|-----------|---------|
| Traces & metrics | OTel SDK batch processor / periodic reader — async, bounded, drop on overflow | always async |
| Alert notifications | `ErrorReporter` background worker: non-blocking enqueue, bounded queue (drops + counts via `NotificationsDropped()`), per-`Notify` timeout, background context | always non-blocking |
| Logging | `NonBlockingHandler` (`OBSERVABILITY_LOG_ASYNC=true`): bounded queue, worker-drained, drops + counts on overflow | sync (opt-in async) |

Under a sustained stall the framework **sheds telemetry, not traffic** — dropped records are
counted and observable, never silent. Logging defaults to synchronous (deterministic for dev
and tests); enable `OBSERVABILITY_LOG_ASYNC` in production where a stalled log sink is
unacceptable.

## Configuration

| Variable | Default | Effect |
|----------|---------|--------|
| `OBSERVABILITY_DRIVER` | `stdout` | `stdout` · `file` · `stack` · `otlp` · `cloudwatch` |
| `OBSERVABILITY_LOG_LEVEL` | `info` | minimum log level |
| `OBSERVABILITY_TRACE_SAMPLE_RATE` | `1.0` | trace sampling `[0..1]` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | — | OTLP collector `host:port` (driver `otlp`) |
| `OTEL_EXPORTER_OTLP_INSECURE` | `false` | TLS on by default; `true` only for local dev |
| `OBSERVABILITY_LOG_ASYNC` | `false` | enable the non-blocking log handler |

See [modules/observability.md](modules/observability.md) for the full driver and field reference.
