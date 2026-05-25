# RFC 0001 — Error Observability & Early-Warning at the Request/Response Layer

| Field | Value |
|-------|-------|
| **Status** | Draft / Proposed |
| **Target version** | `v1.9.0` (additive, backwards-compatible minor) |
| **Created** | 2026-05-26 |
| **Supersedes** | — |
| **Affected modules** | `observability`, `httpx`, `resilience`, `events`, `scheduler`, `messaging/outbox`, `notifications` |
| **Authors** | Pham Thai Duong (with Claude Opus 4.7) |

---

## 1. Abstract

The framework today owns the *building blocks* of error reporting — structured errors
(`httpx.StatusError`, RFC 9457 Problem Details), W3C Trace Context propagation, an OTel
tracer, an slog logger, panic recovery, and per-module `OnError` hooks — but it does **not**
emit a single metric and does **not** connect the request/response error path to the
observability stack. As a result, **operators cannot alert on error rate**, and a handler
that returns an error is, by default, invisible to logs, traces, and metrics.

This RFC specifies a cohesive, standards-based error-observability layer so that every
error — at ingress (request), egress (response), and in background subsystems — is
**detected, classified, recorded, and (optionally) alerted on early**. The design follows
the OpenTelemetry Semantic Conventions, the RED method, and the Google SRE "Four Golden
Signals", and is fully backwards-compatible.

## 2. Motivation

The audit of 2026-05-26 identified five gaps (evidence in parentheses):

- **G1 — No metrics are emitted.** `Provider.Meter()` exists (`observability/provider.go:124`)
  but the codebase creates zero instruments. There is no request counter, **error counter**,
  or latency histogram → **error-rate alerting is impossible**.
- **G2 — The httpx error path is observability-blind.** `writeError` (`httpx/handler.go:58`)
  writes the HTTP response but logs/traces/meters nothing; the Go `error` value (message,
  wrap chain) is discarded. Without separately wiring `observability.HTTP`, errors are silent.
- **G3 — Severity is wrong.** `observability/middleware/http.go:80` logs *every* request,
  including 5xx, at `Info`. Errors cannot be filtered or alerted on by log level.
- **G4 — No error→alert pipeline.** The `OnError`/`OnRun` hooks (`events`, `scheduler`,
  `messaging/outbox`) and the circuit breaker (`resilience/circuitbreaker.go`, which changes
  state **silently**) are not wired to anything. Early alerting must be hand-assembled.
- **G5 — High-cardinality span/metric labels & split middleware.** Span name and `http.target`
  use the raw `r.URL.Path` (`observability/middleware/http.go:57,62`), which explodes
  trace/metric cardinality. The `httpx/middleware` and `observability/middleware` stacks are
  disjoint, with two recover paths and no single instrumented entry point.

## 3. Goals / Non-Goals

### Goals
1. Emit OTel-compliant **RED metrics** for HTTP request/response, enabling error-rate and
   latency-percentile alerting out of the box.
2. Make the **httpx error path observability-aware**: record the error on the active span,
   log it at the correct severity, and classify it.
3. Provide a single **`ErrorReporter`** sink that fans error reports out to logs + metrics +
   (optional) the `notifications` module for early alerting.
4. Make the **circuit breaker observable** (state-change signal → log/metric/event).
5. Eliminate **label cardinality** explosion by using the route template, and offer one
   **instrumented router** that is correct by default.
6. Remain **100% backwards-compatible** (all additive; behavior changes are opt-in).

### Non-Goals
- A bespoke metrics protocol — we use OTel and let the operator's collector (Prometheus,
  Datadog, CloudWatch, …) own storage and alert evaluation.
- Log aggregation / SIEM — out of scope; we emit structured logs only.
- Distributed-tracing UI or an alerting engine — we emit signals; alert *rules* are an
  operator artifact (examples provided in §11).
- Changing the existing public signatures of `Serve`, `writeError`, or `Mutex`.

## 4. Standards & Conventions (normative references)

| Ref | Standard | Use in this design |
|-----|----------|--------------------|
| [OTel HTTP Semconv](https://opentelemetry.io/docs/specs/semconv/http/http-metrics/) | `http.server.request.duration`, `http.server.active_requests`, attrs `http.request.method`, `http.response.status_code`, `http.route`, `error.type` | Metric + span attribute names |
| [OTel Trace Semconv](https://opentelemetry.io/docs/specs/semconv/http/http-spans/) | span name = `{method} {http.route}`; `SetStatus(codes.Error)` for 5xx | Span naming & status |
| RED method (Tom Wilkie) | **R**ate, **E**rrors, **D**uration | Metric selection |
| [Google SRE — Four Golden Signals](https://sre.google/sre-book/monitoring-distributed-systems/) | Latency, Traffic, Errors, Saturation | Dashboard/alert framing |
| [SRE Workbook — Alerting on SLOs](https://sre.google/workbook/alerting-on-slos/) | Multi-window, multi-burn-rate | Alert-rule examples |
| RFC 9457 | Problem Details for HTTP APIs | Already used; error body shape |
| RFC 7231 §6 | HTTP status semantics (4xx client / 5xx server) | Severity classification |
| W3C Trace Context | `traceparent` propagation | Already used; correlation |
| Semantic Versioning 2.0.0 | Additive minor release | `v1.9.0` scoping |

**Cardinality rule (normative):** metric and span attributes MUST use the **route template**
(`/users/{id}`), never the concrete path (`/users/42`). Unmatched routes MUST collapse to a
constant (`"<unmatched>"`). This bounds time-series count and is mandated by OTel semconv.

## 5. Design Overview

```
                       ┌──────────────────── observability.Provider ─────────────────────┐
                       │  Logger (slog)   Tracer (OTel)   Meter (OTel)   ErrorReporter    │
                       └───────▲──────────────▲───────────────▲──────────────▲────────────┘
                               │              │               │              │
   request ──► InstrumentedRouter ──► observability.HTTP ──► httpx.Serve ──► handler
                               │   (trace+metrics+recover)    │ (error-aware)│
   response ◄─────────────────┴──────────────────────────────┴──────────────┘
                                                                      │ err
            background subsystems ──► ErrorReporter.Report(...) ──────┤
   (events.OnError, scheduler.OnRun, outbox.OnError, breaker state)   │
                                                                      ▼
                                        log (severity) + metric (error.type) + notify (alert)
```

The `ErrorReporter` is the new spine. Every error-producing surface reports to it; it fans
out to the three existing primitives plus optional alerting. Nothing is required to change
its call sites destructively — the new path is layered on top.

## 6. Detailed Design

### 6.1 RED metrics (`observability` + `observability/middleware`) — closes G1, G5

Add an instrument set built once from `Provider.Meter()`:

```go
// observability/metrics.go (new)
type HTTPMetrics struct {
    duration       metric.Float64Histogram // http.server.request.duration  (unit "s")
    activeRequests metric.Int64UpDownCounter // http.server.active_requests
}

func NewHTTPMetrics(m metric.Meter) (*HTTPMetrics, error) // registers instruments
```

- **Errors are derived, not separately counted** (OTel-idiomatic): the `duration` histogram
  carries `http.response.status_code` and `error.type`, so error rate = `count{status>=500}`.
  This avoids double-counting and matches every OTel backend's dashboards. *(A convenience
  `http.server.error.count` Int64Counter MAY be added behind a config flag for backends that
  cannot aggregate histograms, but is not the default.)*
- Attributes recorded: `http.request.method`, `http.response.status_code`, `http.route`
  (template), and `error.type` (set to the status text or the Go error type for 5xx).
- `observability/middleware/http.go` is updated to: (a) resolve `http.route` from the chi
  `RouteContext` (`chi.RouteContext(r.Context()).RoutePattern()`), falling back to
  `"<unmatched>"`; (b) name the span `{method} {route}`; (c) `span.SetStatus(codes.Error, …)`
  and set `error.type` on 5xx; (d) `activeRequests.Add(+1/-1)` around the call;
  (e) `duration.Record(elapsed.Seconds(), attrs)`.

### 6.2 Severity-correct request logging — closes G3

Replace the unconditional `InfoContext` with a status→level map:

| Status class | slog level | Rationale (RFC 7231) |
|--------------|-----------|----------------------|
| `2xx`, `3xx` | `Info`    | success / redirect |
| `4xx`        | `Warn`    | client error (often actionable: spikes = abuse/bad client) |
| `5xx`        | `Error`   | server error (always actionable) |

The log record gains `http.route`, `error.type` (when set), `request_id`, and `trace_id`.

### 6.3 Observability-aware httpx error path — closes G2

`httpx.Serve` currently calls `writeError(w, err)`. We add an **optional** error observer that
fires before the response is written, without changing the `Serve`/`writeError` signatures:

```go
// httpx/observe.go (new)
// ErrorObserver is notified of every error returned by a HandlerFunc, before
// the response is written. Set once at startup; nil = today's behavior.
type ErrorObserver func(ctx context.Context, err error, status int)

func SetErrorObserver(fn ErrorObserver)   // process-global, like SetProblemTypeBaseURL
```

`Serve` computes the status from the `*StatusError` (defaulting non-StatusError → 500),
invokes the observer (which records the *actual error value* on the span + logs at the mapped
severity), then writes the response exactly as today. The default observer provided by the
`observability` module bridges to `Provider`/`ErrorReporter`. This recovers the error value
that G2 was dropping while keeping `httpx` free of a hard dependency on `observability`.

### 6.4 Central `ErrorReporter` sink — closes G4

```go
// observability/errorreporter.go (new)
type ErrorReport struct {
    Err      error
    Source   string       // "http" | "scheduler" | "queue" | "events" | "outbox" | "circuitbreaker"
    Severity slog.Level   // default derived from Source/status
    Attrs    []slog.Attr  // structured context (job name, subject, route, …)
}

type ErrorReporter interface {
    Report(ctx context.Context, r ErrorReport)
}
```

Default implementation (`Provider`-backed) fans out to:
1. **Log** at `r.Severity` with `error.type`, `trace_id`, `source`, and `Attrs`.
2. **Metric** `errors.reported` (Int64Counter) keyed by `source` + `error.type`.
3. **Notifications (optional, rate-limited):** if a notifier + a severity threshold are
   configured, dispatch via the `notifications` module (Slack/Discord/webhook/mail). A
   token-bucket limiter (per `source`+`error.type`) prevents alert storms.

Wiring (all additive, opt-in): adapters convert existing hooks into reports —
`events.OnError`, `scheduler.OnRun(job, err)`, `messaging/outbox.OnError`, and the breaker
callback (§6.5) — via tiny `func(error)` shims provided by the `observability` module.

### 6.5 Observable circuit breaker — closes G4

Extend `resilience.CircuitBreakerConfig` additively:

```go
type CircuitBreakerConfig struct {
    MaxFailures   int
    ResetTimeout  time.Duration
    OnStateChange func(from, to State) // NEW, optional; nil = today's behavior
}
type State int // Closed, Open, HalfOpen
```

`httpclient` (its primary consumer) wires `OnStateChange` to an `ErrorReporter.Report` with
`Source:"circuitbreaker"` and `Severity: Warn` (Open) / `Info` (Closed), plus an
`circuitbreaker.state` gauge. A breaker opening is a leading indicator — surfacing it is the
"early warning" the audit calls for.

### 6.6 `InstrumentedRouter` — closes G5

A single constructor that composes the correct defaults so services don't hand-wire four
middlewares:

```go
// observability/middleware/router.go (new) or httpx helper
func InstrumentedRouter(p *observability.Provider, opts ...httpx.RouterOptions) *chi.Mux
// = chi router + RequestID + observability.HTTP (trace+metrics+severity) + Recover
//   (single recover path; chi RealIP stays opt-in per the v1.8 ratelimit fix)
```

## 7. Public API summary (all additive)

| Symbol | Package | Kind |
|--------|---------|------|
| `HTTPMetrics`, `NewHTTPMetrics` | `observability` | type/ctor |
| `ErrorReport`, `ErrorReporter`, default reporter ctor | `observability` | type/iface |
| `Provider.ErrorReporter()` / `Provider.HTTPMetrics()` | `observability` | accessor |
| `ErrorObserver`, `SetErrorObserver` | `httpx` | type/func |
| `InstrumentedRouter` | `observability/middleware` | func |
| `CircuitBreakerConfig.OnStateChange`, `State` | `resilience` | field/type |

No existing symbol changes shape. `go build ./...` for current consumers is unaffected.

## 8. Configuration (env, additive)

| Variable | Default | Effect |
|----------|---------|--------|
| `OBSERVABILITY_METRICS_ENABLED` | `true` | register/record HTTP RED metrics |
| `OBSERVABILITY_ERROR_NOTIFY_CHANNEL` | `""` | notifications channel for alerting (empty = off) |
| `OBSERVABILITY_ERROR_NOTIFY_MIN_LEVEL` | `error` | min severity that triggers a notification |
| `OBSERVABILITY_ERROR_NOTIFY_RATE` | `1/min` | token-bucket rate per `source`+`error.type` |

Secure/standards defaults: metrics on, alerting off until a channel is set (no surprise egress).

## 9. Backwards compatibility & migration

- All additions are new symbols or new optional struct fields → no breaking change; SemVer
  **minor** (`v1.9.0`).
- Services on the existing `observability.HTTP` middleware automatically gain RED metrics and
  correct log severity (behavioral improvement, not an API break). Span names change from raw
  path to route template — **this is the intended cardinality fix**; documented in the
  changelog as it affects existing trace/dashboard queries.
- `ErrorObserver`, `ErrorReporter`, alerting, and the breaker callback are **opt-in**; doing
  nothing preserves today's behavior exactly.

## 10. Testing strategy

| Layer | Test |
|-------|------|
| Metrics | `go.opentelemetry.io/otel/sdk/metric/metricdata` + manual reader: assert `http.server.request.duration` recorded with correct `http.route`/`status_code`/`error.type`; assert `active_requests` returns to 0. |
| Cardinality | request to an unmatched path records `http.route="<unmatched>"` (no per-path series). |
| Severity | table test: 200→Info, 404→Warn, 500→Error (capture slog). |
| httpx error path | handler returning a wrapped error → observer sees the *error value* + status; span has `codes.Error`; response unchanged (regression vs current `writeError`). |
| ErrorReporter | report fans out to log + counter + (mock) notifier; rate-limiter caps notifications. |
| Breaker | `OnStateChange` fires Closed→Open after `MaxFailures`; reporter receives it. |
| Race | `go test -race` across `observability`, `httpx`, `resilience`. |

## 11. Alerting playbook (operator artifact, example)

With an OTel→Prometheus pipeline, the derived RED metrics enable standard SRE alerts:

```promql
# Fast-burn (page): 5xx ratio > 5% over 5m AND 1h  (multi-window, multi-burn-rate)
sum(rate(http_server_request_duration_seconds_count{http_response_status_code=~"5.."}[5m]))
  / sum(rate(http_server_request_duration_seconds_count[5m])) > 0.05

# Latency SLO: p99 > 500ms over 10m
histogram_quantile(0.99,
  sum by (le) (rate(http_server_request_duration_seconds_bucket[10m]))) > 0.5

# Early warning: circuit breaker open (leading indicator)
max(circuitbreaker_state) >= 1
```

These rules live in the operator's monitoring repo, not in the framework; they are documented
here so the metric *shape* is designed to make them expressible.

## 12. Security & performance

- **Cardinality** (§4) is the primary risk; the route-template rule bounds series count.
- **No PII in labels:** only method/route/status/error-type; never user IDs or query strings.
- **Alert egress** is off by default and rate-limited when on (prevents alert storms and
  accidental data exfil through notification payloads — consistent with the v1.8 SSRF guard).
- **Hot-path cost:** one histogram record + one up-down counter add per request; instruments
  are created once at startup, not per request. Negligible vs handler cost.

## 13. Phased rollout

| Phase | Scope | PR / commit prefix |
|-------|-------|--------------------|
| **P1** ✅ *implemented* | RED metrics + route-template span/labels + severity mapping in `observability.HTTP` | `feat(observability):` |
| **P2** | `httpx.ErrorObserver` + observability bridge + `InstrumentedRouter` | `feat(httpx):` / `feat(observability):` |
| **P3** | `ErrorReporter` sink + adapters for events/scheduler/outbox | `feat(observability):` |
| **P4** | Circuit-breaker `OnStateChange` + notifications alerting bridge | `feat(resilience):` / `feat(observability):` |
| **P5** | Docs: `docs/modules/observability.md`, `httpx.md`, `OBSERVABILITY.md` guide + this RFC → `Accepted` | `docs:` |

Each phase is independently shippable and backwards-compatible. P1 alone closes the most
critical gap (no metrics → no alerting).

## 14. Open questions

1. Convenience `http.server.error.count` counter on by default, or histogram-derived only? (Lean: derived-only, flag for the counter.)
2. Should `messaging`/`queue` consume `ErrorReporter` here, or in a follow-up RFC, given those areas have concurrent ownership?
3. Notification alerting: ship a default formatter, or require the caller to provide one?

## 15. References

See §4. Implementation maps to: `observability/provider.go:124`, `observability/middleware/http.go`,
`httpx/handler.go:50-70`, `resilience/circuitbreaker.go`, `events/async.go:111`,
`scheduler/scheduler.go:343`, `messaging/outbox/poller.go:38`.
