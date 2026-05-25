# HTTP Client

> Swappable HTTP client drivers with OTel client spans, retry/circuit-breaker, and test mocks.

## Concepts

A `Manager` owns named `Client` handles. Each client wraps a driver (`stdlib`, `mock`, `resilient`). Application code calls `Get` / `Post` / `PostJSON` — never imports a driver directly except in tests.

```
Manager ── named Clients
   └─ Client.Get/Post/Do ── driver (stdlib · mock · resilient)
         └─ middleware.InstrumentTransport (OTel client spans)
```

## Quick start

```go
app := framework.New("svc", "1.0.0").Use(httpclient.Module)
_ = app.Init(ctx)
mgr, _ := httpclient.FromApp(app)
resp, err := mgr.Get(ctx, "https://api.example/users/1")
if err != nil { /* ... */ }
defer resp.Body.Close()
```

With `HTTPCLIENT_BASE_URL` set, relative paths work:

```bash
HTTPCLIENT_BASE_URL=https://api.example
```

```go
resp, _ := mgr.Get(ctx, "/users/1")  // → https://api.example/users/1
```

## Drivers

| Driver | Registration | When to use |
|--------|--------------|-------------|
| `stdlib` | auto | Default production outbound HTTP + OTel spans |
| `mock` | auto | Unit tests — record requests, queue responses |
| `resilient` | auto | Outbound calls that need retry + circuit breaker |

### stdlib

Wraps `net/http.Client` with timeout, default headers, base URL, and OTel-instrumented transport (`http.method`, `http.url`, status on span).

```bash
HTTPCLIENT_DEFAULT=stdlib
HTTPCLIENT_TIMEOUT=30s
HTTPCLIENT_BASE_URL=https://hooks.slack.com
HTTPCLIENT_OTEL_SERVICE=billing-api-outbound
```

### mock (tests)

```go
import (
    "github.com/godx-jp/godx-platform-framework/httpclient/drivers/mock"
    hdriver "github.com/godx-jp/godx-platform-framework/httpclient/driver"
)

raw, _ := hdriver.New(ctx, hdriver.Spec{Name: hdriver.DriverMock})
m := raw.(*mock.Client)
m.PushResponse(200, []byte(`{"ok":true}`), nil)

c := httpclient.Wrap(raw)
resp, _ := c.Get(ctx, "https://example/api")
```

### resilient

Wraps stdlib with:

- Retry on 5xx for idempotent methods (GET, HEAD, PUT, DELETE, …)
- Exponential backoff + jitter
- Circuit breaker (opens after N failures, half-open retry)

Uses `resilience/` package internally (v0.13.2+).

```bash
HTTPCLIENT_DEFAULT=resilient
HTTPCLIENT_MAX_RETRIES=3
HTTPCLIENT_RETRY_BACKOFF=100ms
```

## Env vars

See [CONFIGURATION.md](../CONFIGURATION.md#http-client).

## Use cases

### Call external REST API

```go
resp, err := mgr.Default().Get(ctx, "https://api.stripe.com/v1/charges/ch_123")
```

Set `Authorization` via default headers in config or per-request:

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

### Resilient partner integration

```bash
HTTPCLIENT_CLIENTS=partner
HTTPCLIENT_DEFAULT=partner
# configure partner store with resilient driver via ModuleWithConfig
```

Programmatic:

```go
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
            },
        },
    },
}
```

### Test without network

Use `mock` driver in tests; assert `m.Requests()` after handler runs.

## Laravel mapping

| Laravel | Framework |
|---------|-----------|
| `Http::get(...)` | `mgr.Get(ctx, url)` |
| `Http::post(...)` | `mgr.Default().Post(ctx, path, body, contentType)` |
| `Http::retry(3, 100, fn)` | `resilient` driver or `resilience.Retry` |
| `Http::fake()` | `mock` driver + `PushResponse` |

## Tests

| File | Covers |
|------|--------|
| `driver/registry_test.go` | Registry |
| `drivers/stdlib/stdlib_test.go` | httptest.Server round trip |
| `drivers/mock/mock_test.go` | Record + respond |
| `module_test.go` | App wiring |

```bash
go test -race ./httpclient/...
```

## Migrating from go-common

Replace bespoke `http.Client` wrappers with `httpclient.Module`. Enable OTel by using the default stdlib driver (spans emit automatically when the global OTel SDK is configured via `observability.Module`).
