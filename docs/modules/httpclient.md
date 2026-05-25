# HTTP Client

> Swappable HTTP client drivers with OTel client spans, retry/circuit-breaker, and test mocks.

## Quick start

```go
app := framework.New("svc", "1.0.0").Use(httpclient.Module)
_ = app.Init(ctx)
mgr, _ := httpclient.FromApp(app)
resp, err := mgr.Get(ctx, "https://api.example/users/1")
```

## Drivers

| Driver | Registration | Notes |
|--------|--------------|-------|
| `stdlib` | auto | Default `net/http.Client` + OTel transport |
| `mock` | auto | Records requests; `PushResponse` for tests |
| `resilient` | auto | Retry + backoff + circuit breaker on stdlib |

## Env vars

See [CONFIGURATION.md](../CONFIGURATION.md#http-client).

## Laravel mapping

| Laravel | Framework |
|---------|-----------|
| `Http::get(...)` | `mgr.Get(ctx, url)` |
| `Http::post(...)` | `mgr.Default().PostJSON(ctx, path, body)` |
