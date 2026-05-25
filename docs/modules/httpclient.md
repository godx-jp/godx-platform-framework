# HTTP Client

> Laravel-style HTTP client facade with swappable drivers (`stdlib`,
> `mock`, `resilient`), automatic OpenTelemetry client spans on every
> outbound request, and convenience helpers (`Get`, `Post`, `PostJSON`).

## Concepts

A `Manager` owns named `Client` handles. Each `Client` wraps a `driver.Client` (`stdlib`, `mock`, `resilient`) and adds base-URL resolution plus `Get`/`Post`/`PostJSON` helpers. Application code calls those helpers and never imports a driver directly except in tests. The `resilient` driver layers retry, backoff, and a circuit breaker on top of `stdlib` using the [resilience](resilience.md) module.

```
Manager ── named Clients
   └─ Client.Get/Post/PostJSON/Do ── driver.Client (stdlib · mock · resilient)
         └─ middleware.InstrumentTransport (OTel client spans)
```

## Quick start

```go
package main

import (
    "context"

    "github.com/godx-jp/godx-platform-framework/framework"
    "github.com/godx-jp/godx-platform-framework/httpclient"
)

func main() {
    ctx := context.Background()
    app := framework.New("svc", "1.0.0").Use(httpclient.Module)
    if err := app.Init(ctx); err != nil { panic(err) }
    defer app.Shutdown(ctx)

    mgr, _ := httpclient.FromApp(app)
    resp, err := mgr.Get(ctx, "https://api.example/users/1")
    if err != nil { panic(err) }
    defer resp.Body.Close()
}
```

With nothing configured you get a single `stdlib` client named `stdlib` with a 30 s timeout. With `HTTPCLIENT_BASE_URL` set, relative paths resolve against it:

```bash
HTTPCLIENT_BASE_URL=https://api.example
```

```go
resp, _ := mgr.Get(ctx, "/users/1")  // → https://api.example/users/1
```

## Env-var config

| Variable | Default | Purpose |
|---|---|---|
| `HTTPCLIENT_DEFAULT` | `stdlib` | Default client / driver name |
| `HTTPCLIENT_CLIENTS` | _default only_ | Comma-separated client names (each becomes a client whose driver is its name) |
| `HTTPCLIENT_BASE_URL` | _empty_ | Base URL prepended to relative request paths |
| `HTTPCLIENT_TIMEOUT` | `30s` | Per-request timeout (Go duration) |
| `HTTPCLIENT_OTEL_SERVICE` | `httpclient` | Tracer / span service name |
| `HTTPCLIENT_MAX_RETRIES` | `3` | `resilient`: max retries (attempts = retries + 1) |
| `HTTPCLIENT_RETRY_BACKOFF` | `100ms` | `resilient`: base backoff between retries |

The env loader maps `HTTPCLIENT_CLIENTS` entries one-to-one to drivers by name, so `HTTPCLIENT_CLIENTS=resilient` builds a `resilient` client. For finer control (multiple distinct clients, per-client base URLs, circuit-breaker tuning) use programmatic config.

## Programmatic config

```go
import (
    "time"

    "github.com/godx-jp/godx-platform-framework/framework"
    "github.com/godx-jp/godx-platform-framework/httpclient"
    hdriver "github.com/godx-jp/godx-platform-framework/httpclient/driver"
)

cfg := httpclient.Config{
    Default: "partner",
    Clients: map[string]httpclient.ClientConfig{
        "partner": {
            Driver: hdriver.DriverResilient,
            Spec: hdriver.Spec{
                Name:           hdriver.DriverResilient,
                BaseURL:        "https://partner.example",
                Timeout:        10 * time.Second,
                MaxRetries:     3,
                RetryBackoff:   200 * time.Millisecond,
                CBMaxFailures:  5,
                CBResetTimeout: 30 * time.Second,
                DefaultHeaders: map[string]string{"X-Api-Key": "..."},
            },
        },
    },
}
app := framework.New("svc", "1.0.0").Use(httpclient.ModuleWithConfig(cfg))
```

`Config.Validate` requires a non-empty `Default`, at least one client, and the default name to be present in `Clients`. A `ClientConfig` with an empty `Spec.Name` inherits its `Driver` as the spec name at build time, and each built client is wrapped with its `Spec.BaseURL`.

## Client / Manager API

`Manager.Get` is a convenience that targets the default client. Everything else goes through the `*Client` handle.

| Method | Laravel parallel |
|---|---|
| `mgr.Get(ctx, path) (*http.Response, error)` | `Http::get($url)` — uses the default client |
| `mgr.Default() *Client` / `mgr.Client(name) (*Client, error)` | `Http::client(...)` |
| `mgr.Names() []string` | sorted client names |
| `c.Get(ctx, path) (*http.Response, error)` | `Http::get($url)` |
| `c.Post(ctx, path, body io.Reader, contentType string) (*http.Response, error)` | `Http::post($url, ...)` |
| `c.PostJSON(ctx, path, v any) (*http.Response, error)` | `Http::post($url, $json)` — marshals `v` and sets `application/json` |
| `c.Do(ctx, req *http.Request) (*http.Response, error)` | escape hatch for full `*http.Request` control |

`httpclient.Wrap(driverClient)` and `httpclient.WrapWithBase(driverClient, base)` build a `*Client` from a raw `driver.Client` (used in tests). `httpclient.JSON` is the `application/json` content-type constant. Path resolution: absolute `http(s)://` paths pass through unchanged; otherwise the client's base URL is prepended.

## Driver matrix

| Driver | Status | Registration | Notes |
|---|---|---|---|
| `stdlib` | stable | auto (light) | `net/http.Client` wrapper with timeout, default headers, base URL, and OTel-instrumented transport |
| `mock` | stable | auto (light) | In-memory recorder for tests — queue responses, inspect recorded requests; unmatched calls return `404` |
| `resilient` | stable | auto (light) | Wraps `stdlib` with retry + backoff + jitter + circuit breaker via the `resilience` module |

All three register via blank imports in the package's `register.go`, so importing `httpclient` makes every driver available — no opt-in blank import is needed. `resilient` pulls in the in-tree `resilience` package but no external SDK, so it is still treated as light.

### stdlib

```bash
HTTPCLIENT_DEFAULT=stdlib
HTTPCLIENT_TIMEOUT=30s
HTTPCLIENT_BASE_URL=https://hooks.slack.com
HTTPCLIENT_OTEL_SERVICE=billing-api-outbound
```

Wraps `net/http.Client`. Default headers are applied only when the request doesn't already set them. A closed client returns `driver.ErrClosed`.

### mock (tests)

The mock driver's exported constructor is `mock.New()`, which returns a value exposing `PushResponse` and `Requests`. Build it directly and wrap it:

```go
import (
    "github.com/godx-jp/godx-platform-framework/httpclient"
    "github.com/godx-jp/godx-platform-framework/httpclient/drivers/mock"
)

m := mock.New()
m.PushResponse(200, []byte(`{"ok":true}`), nil)

c := httpclient.Wrap(m)
resp, _ := c.Get(ctx, "https://example/api")

// assert what the handler sent:
reqs := m.Requests()
```

Responses are returned in FIFO order from the queue; once the queue is drained, further calls return `404 "not found"`.

### resilient

Wraps `stdlib` with:

- Retry on transport errors and on `5xx` responses, **only for the RFC 7231 "safe" methods** (`GET`, `HEAD`, `OPTIONS`, `TRACE`).
- Exponential backoff with jitter (`MaxAttempts = MaxRetries + 1`).
- A circuit breaker that opens after `CBMaxFailures` consecutive failures and half-opens after `CBResetTimeout`.

> **Security — retries are scoped to safe methods.** Non-safe methods (`POST`, `PUT`, `PATCH`, `DELETE`) are **never retried**, neither on a `5xx` response nor on a transport error. Retrying them could duplicate side effects (a second write/charge/delete) and multiply load on a struggling upstream, since the request may already have reached the server. Such failures still count against the circuit breaker but are returned to the caller after a single attempt.

```bash
HTTPCLIENT_DEFAULT=resilient
HTTPCLIENT_MAX_RETRIES=3
HTTPCLIENT_RETRY_BACKOFF=100ms
```

Defaults when unset: `MaxRetries=3`, `RetryBackoff=100ms`, `CBMaxFailures=5`, `CBResetTimeout=30s`. The circuit-breaker thresholds are only settable through programmatic config (`Spec.CBMaxFailures` / `Spec.CBResetTimeout`). When the breaker is open, `Do` returns `driver.ErrCircuitOpen`. Built on the [resilience](resilience.md) module (`resilience.Do`, `resilience.CircuitBreaker`).

## OTel instrumentation

Every `stdlib`-backed request (including via `resilient`) goes through `middleware.InstrumentTransport`, which starts a client-kind span named `<METHOD> <host><path>`, sets `http.request.method` / `url.full` / `http.status_code` attributes, marks `5xx` as an error status, and injects W3C `traceparent` headers for context propagation. The tracer is named by `HTTPCLIENT_OTEL_SERVICE` (default `httpclient`). Spans are emitted whenever a global OTel SDK is configured (e.g. via `observability.Module`); without one they are no-ops.

## Error model

| Sentinel (`httpclient/driver`) | Meaning |
|---|---|
| `driver.ErrClosed` | the client has been shut down |
| `driver.ErrCircuitOpen` | the `resilient` breaker is open |
| `driver.ErrInvalidBaseURL` | base URL failed to parse |

`mgr.Get` returns `httpclient: no default client` when no default is set. Transport-level errors from `net/http` propagate unchanged; the `resilient` driver translates `resilience.ErrOpen` into `driver.ErrCircuitOpen`.

## Use cases

### Call external REST API with auth

```go
req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
req.Header.Set("Authorization", "Bearer "+token)
resp, err := mgr.Default().Do(ctx, req)
```

### POST JSON webhook

```go
payload := map[string]any{"event": "order.paid", "id": orderID}
resp, err := mgr.Default().PostJSON(ctx, "https://partner.example/hook", payload)
```

## Laravel mapping

| Laravel | Framework |
|---|---|
| `Http::get(...)` | `mgr.Get(ctx, url)` |
| `Http::post(...)` | `mgr.Default().Post(ctx, path, body, contentType)` |
| `Http::retry(3, 100, fn)` | `resilient` driver (or `resilience.Do` directly) |
| `Http::fake()` | `mock` driver + `PushResponse` |

## Context propagation

`httpclient.ContextWithManager(ctx, mgr)` attaches a `*Manager` to a context; `httpclient.FromContext(ctx)` retrieves it (`ok == false` when absent). `httpclient.FromApp(app)` is the canonical way to retrieve the manager built by `httpclient.Module`. The per-request `context.Context` you pass to `Get`/`Post`/`Do` drives both the HTTP timeout and OTel trace propagation.

## Lifecycle

`httpclient.Module` registers `Manager.Shutdown` as an `OnShutdown` callback, which calls `Shutdown` on every client (joining any errors). Only one `httpclient.Module` may be initialised per App.
