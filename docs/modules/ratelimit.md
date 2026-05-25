# Rate limit

> Token-bucket rate limiting with in-process and Redis backends plus HTTP middleware that returns 429 + Retry-After.

## Concepts

A `Manager` holds one or more named `Limiter` instances. Each limiter is a token bucket: `Allow(ctx, key)` consumes one token; when the bucket is empty you get `ok == false`. Keys are **opaque strings** you choose (`user:42`, `ip:203.0.113.1`, `tenant:acme:api`).

```
Manager ── named Limiters
   └─ Limiter.Allow(ctx, key) ── driver (memory · redis)
```

## Quick start

```go
app := framework.New("svc", "1.0.0").Use(ratelimit.Module)
_ = app.Init(ctx)
mgr, _ := ratelimit.FromApp(app)
ok, err := mgr.Allow(ctx, "user:42")
if !ok {
    // rate limited
}
```

Default driver is **memory** (single process). For multiple pods, blank-import Redis (see [Shared Redis](../SHARED_INFRA.md#one-redis-server-for-everything-recommended)).

## HTTP middleware

```go
import (
    "github.com/godx-jp/godx-platform-framework/ratelimit/middleware"
)

lim := mgr.Default()
handler := middleware.Limit(lim, middleware.ByIP)(yourHandler)

// API key tier:
handler = middleware.Limit(lim, middleware.ByHeader("X-API-Key"))(handler)
```

When the bucket is empty: **429 Too Many Requests** + **Retry-After** (seconds until a token refills).

Compose with chi / httpx:

```go
r.Use(middleware.Limit(lim, middleware.ByIP))
```

## Drivers

| Driver | Registration | Notes |
|--------|--------------|-------|
| `memory` | auto | `sync.Map` of token buckets; dev / single replica |
| `redis` | blank import | Lua atomic token bucket; **same Redis URL as cache OK** — use a different prefix |

```go
import _ "github.com/godx-jp/godx-platform-framework/ratelimit/drivers/redis"
```

### Shared Redis with cache (Laravel-style)

**One Redis server.** Different prefixes — no key collision:

```bash
REDIS_URL=redis://:secret@127.0.0.1:6379/0

# cache keys: myapp:cache:...
CACHE_STORE_PRIMARY_URL=${REDIS_URL}
CACHE_STORE_PRIMARY_PREFIX=cache:

# ratelimit keys: myapp:ratelimit:...
RATELIMIT_LIMITER_API_URL=${REDIS_URL}
RATELIMIT_LIMITER_API_PREFIX=ratelimit:
```

See [SHARED_INFRA.md](../SHARED_INFRA.md) for the full production sketch.

## Env vars

| Variable | Default | Purpose |
|----------|---------|---------|
| `RATELIMIT_DEFAULT` | `memory` | Default limiter name |
| `RATELIMIT_LIMITERS` | default only | CSV of limiter names |
| `RATELIMIT_PREFIX` | _empty_ | Global Redis key prefix |
| `RATELIMIT_LIMITER_<NAME>_RATE` | `60` | Tokens per window |
| `RATELIMIT_LIMITER_<NAME>_BURST` | `rate` | Bucket capacity |
| `RATELIMIT_LIMITER_<NAME>_URL` | | Redis URL (redis driver) |

Full list: [CONFIGURATION.md](../CONFIGURATION.md#rate-limit).

## Use cases

### Per-IP public API (100/min)

```bash
RATELIMIT_LIMITERS=api
RATELIMIT_LIMITER_API_DRIVER=redis
RATELIMIT_LIMITER_API_RATE=100
RATELIMIT_LIMITER_API_BURST=100
RATELIMIT_LIMITER_API_URL=redis://127.0.0.1:6379/0
RATELIMIT_LIMITER_API_PREFIX=myapp:ratelimit:
```

```go
handler := middleware.Limit(mgr.Default(), middleware.ByIP)(apiHandler)
```

### Per-user authenticated API

```go
keyFn := func(r *http.Request) string {
    uid, _ := auth.UserIDFromContext(r.Context())
    return "user:" + uid
}
handler := middleware.Limit(lim, keyFn)(handler)
```

### Login brute-force (5 attempts / 15 min)

```go
ok, _ := mgr.Allow(ctx, "login:"+email)
if !ok {
    return ErrTooManyAttempts
}
```

Use a dedicated limiter with low rate via `ModuleWithConfig` or `RATELIMIT_LIMITER_LOGIN_*` env vars.

### Dev / tests — memory only

```bash
RATELIMIT_LIMITERS=memory
# no Redis import needed
```

## API reference

| Method | Description |
|--------|-------------|
| `mgr.Allow(ctx, key) (bool, error)` | Consume one token |
| `mgr.Reset(ctx, key) error` | Clear bucket for key |
| `lim.Allow / Reset` | Same on a named limiter |
| `mgr.Default()` | Default limiter |
| `mgr.Limiter(name)` | Named limiter |

## Laravel mapping

| Laravel | Framework |
|---------|-----------|
| `RateLimiter::attempt(...)` | `mgr.Allow(ctx, key)` |
| `RateLimiter::clear(...)` | `mgr.Reset(ctx, key)` |
| `throttle:api` middleware | `middleware.Limit(lim, middleware.ByIP)` |
| Redis store in `RateLimiter` | `ratelimit` redis driver, same server as cache |

## Tests

| File | Covers |
|------|--------|
| `driver/registry_test.go` | Registry contract |
| `conformance_test.go` | Memory: burst, deny, reset, isolation |
| `middleware/http_test.go` | 429, Retry-After, ByHeader |
| `module_test.go` | App wiring, env defaults |

```bash
go test -race ./ratelimit/...
```

## Migrating from go-common

Replace ad-hoc `sync.Map` counters or bespoke Redis Lua with the driver registry. Use **memory** locally; blank-import **redis** when replicas must share one limit. Point at the **same Redis URL** as cache with a **different prefix** — do not run a second Redis server unless ops requires it.
