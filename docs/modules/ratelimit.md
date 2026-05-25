# Rate limit

> Token-bucket rate limiting with in-process and Redis backends plus HTTP middleware that returns 429 + Retry-After.

## Quick start

```go
app := framework.New("svc", "1.0.0").Use(ratelimit.Module)
_ = app.Init(ctx)
mgr, _ := ratelimit.FromApp(app)
ok, err := mgr.Allow(ctx, "user:42")
```

## HTTP middleware

```go
lim := mgr.Default()
handler := middleware.Limit(lim, middleware.ByIP)(yourHandler)
// or rate-limit by header:
handler := middleware.Limit(lim, middleware.ByHeader("X-API-Key"))(yourHandler)
```

When the bucket is empty the middleware responds with **429 Too Many Requests** and a **Retry-After** header (seconds).

## Drivers

| Driver | Registration | Notes |
|--------|--------------|-------|
| `memory` | auto | Per-key token buckets in a `sync.Map`; single process |
| `redis` | blank import | Distributed token bucket via Lua; shared across replicas |

```go
import _ "github.com/godx-jp/godx-platform-framework/ratelimit/drivers/redis"
```

## Env vars

See [CONFIGURATION.md](../CONFIGURATION.md#rate-limit).

## Laravel mapping

| Laravel | Framework |
|---------|-----------|
| `RateLimiter::attempt(...)` | `mgr.Allow(ctx, key)` |
| `RateLimiter::clear(...)` | `mgr.Reset(ctx, key)` or `lim.Reset(ctx, key)` |
| `throttle` middleware | `middleware.Limit(lim, middleware.ByIP)` |

## Migrating from go-common

Replace ad-hoc `sync.Map` counters or bespoke Redis scripts with the driver registry. Pick `memory` for dev/tests and blank-import `redis` when multiple replicas must share one limit.
