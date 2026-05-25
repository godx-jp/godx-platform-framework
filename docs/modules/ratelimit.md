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

When the bucket is empty: **429 Too Many Requests** + **Retry-After**
(seconds, ceil of the configured `RetryAfter`, default `1s`). A limiter
**error** maps to **500 Internal Server Error**.

Compose with chi / httpx:

```go
r.Use(middleware.Limit(lim, middleware.ByIP))
```

### Middleware API

| Symbol | Purpose |
|---|---|
| `Handler(Options) func(http.Handler) http.Handler` | Full control via `Options{Limiter, KeyFunc, RetryAfter}` |
| `Limit(l, keyFunc) func(http.Handler) http.Handler` | Convenience wrapper, default `RetryAfter` |
| `LimitWithRetryAfter(l, keyFunc, d) func(http.Handler) http.Handler` | `Limit` with an explicit `Retry-After` duration |
| `ByIP(*http.Request) string` | Key by the connection peer (`r.RemoteAddr` host). **Ignores `X-Forwarded-For` / `X-Real-IP`** |
| `ByForwardedFor(trustedProxies []netip.Prefix) KeyFunc` | Key by the real client from `X-Forwarded-For`, honoured **only** behind a trusted proxy |
| `ByHeader(name string) KeyFunc` | Key by a request header value |
| `UserKey(header string) KeyFunc` | `user:<header>`, falls back to `ByIP` when the header is empty |
| `StringKey(parts ...string) string` | Join key segments with `:` |

`KeyFunc` is `func(*http.Request) string`. When the resolved key is
empty the middleware uses the literal `"_"`. `Handler` defaults a nil
`KeyFunc` to `ByIP` and a non-positive `RetryAfter` to `1s`.

### Client IP: safe by default

`ByIP` keys on the **connection peer** (`r.RemoteAddr`) and deliberately
ignores the client-supplied `X-Forwarded-For` / `X-Real-IP` headers. Those
headers are trivially spoofed; honouring them lets an attacker mint a fresh
rate-limit bucket per forged IP and bypass the limit entirely. `UserKey`
inherits this safe fallback. Note the same reasoning applies to chi's
`RealIP` middleware — `httpx.NewRouter` no longer enables it by default.

For deployments **behind a trusted reverse proxy or load balancer**, use
`ByForwardedFor` with the CIDRs of your own proxies:

```go
trusted := []netip.Prefix{
    netip.MustParsePrefix("10.0.0.0/8"),    // internal LB subnet
    netip.MustParsePrefix("172.16.0.0/12"),
}
handler := middleware.Limit(lim, middleware.ByForwardedFor(trusted))(yourHandler)
```

`ByForwardedFor` honours `X-Forwarded-For` **only** when the connection peer
is itself within `trustedProxies`; it then walks the chain right-to-left and
returns the right-most hop that is *not* a trusted proxy — the real client as
seen by your edge. If the peer is not a trusted proxy, the header is ignored
and it falls back to the peer address. Never pass an empty/over-broad
`trustedProxies` at an internet-facing edge — that re-opens the spoofing hole.

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
| `RATELIMIT_LIMITERS` | the default name | CSV of limiter names |
| `RATELIMIT_RATE` | `10` | Global default refill rate (tokens/sec) |
| `RATELIMIT_BURST` | `20` | Global default bucket capacity |
| `RATELIMIT_PREFIX` | _empty_ | Global Redis key prefix |
| `RATELIMIT_LIMITER_<NAME>_DRIVER` | inferred from name | Backend (`memory` / `redis`) |
| `RATELIMIT_LIMITER_<NAME>_RATE` | `RATELIMIT_RATE` | Refill rate (tokens/sec) |
| `RATELIMIT_LIMITER_<NAME>_BURST` | `RATELIMIT_BURST` | Bucket capacity |
| `RATELIMIT_LIMITER_<NAME>_PREFIX` | `RATELIMIT_PREFIX` | Redis key prefix |
| `RATELIMIT_LIMITER_<NAME>_URL` | — | Redis URL (`redis://…`) |
| `RATELIMIT_LIMITER_<NAME>_ADDRESS` | — | Redis host:port when URL unset |
| `RATELIMIT_LIMITER_<NAME>_USERNAME` | — | Redis username |
| `RATELIMIT_LIMITER_<NAME>_PASSWORD` | — | Redis password |
| `RATELIMIT_LIMITER_<NAME>_DB` | `0` | Redis logical DB |

When a limiter is named `memory` or `redis` the driver is inferred from
the name; otherwise set `_DRIVER` explicitly. `<NAME>` is upper-cased
with `-` replaced by `_`.

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
| `mgr.Allow(ctx, key) (bool, error)` | Consume one token from the default limiter |
| `mgr.Reset(ctx, key)` | Clear the bucket for key on the default limiter (no return value) |
| `mgr.Default() driver.Limiter` | Default limiter (may be nil if unset) |
| `mgr.Limiter(name) (driver.Limiter, error)` | Named limiter |
| `mgr.Names() []string` | Sorted limiter names |
| `lim.Allow(ctx, key) (bool, error)` | Consume one token on a named limiter |
| `lim.Reset(ctx, key)` | Clear bucket on a named limiter (no return value) |

The `driver.Limiter` interface is `Name() string`,
`Allow(ctx, key) (bool, error)`, `Reset(ctx, key)`, and
`Shutdown(ctx) error`.

## Error model

| Error | When |
|-------|------|
| `driver.ErrClosed` | A limiter is used after `Shutdown` |

`mgr.Allow` returns an error (not `false`) when no default limiter is
configured. `mgr.Limiter(name)` errors on an unknown name. The Redis
driver surfaces connection/Lua errors through `Allow`; the middleware
turns those into HTTP 500.

## Context propagation

`ratelimit.ContextWithManager(ctx, mgr)` attaches a manager to a context
and `ratelimit.FromContext(ctx)` retrieves it. `ratelimit.FromApp(app)`
is the canonical way to fetch the manager built by `ratelimit.Module`.

## Lifecycle

`ratelimit.Module` stores the `Manager` under `ratelimit.StoreKey` and
registers an `OnShutdown` callback that calls `Shutdown` on every
limiter (closing the Redis pool, marking memory limiters closed). Errors
from individual limiters are joined and returned.

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
