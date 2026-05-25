# Shared infrastructure — Redis, drivers, Laravel parity

How to wire **one** Redis (or one Postgres, one SMTP relay, …) across multiple framework modules without key collisions — the same mental model as Laravel's `config/database.php` + prefixed connections.

## One Redis server for everything (recommended)

**You do not need a separate Redis instance per module.**

Laravel runs cache, sessions, queues, and rate limits against the **same Redis host** by default, separating concerns with:

- **Key prefixes** (`cache:`, `horizon:`, …), and/or
- **Logical DB index** (`redis://host:6379/0` vs `/1`)

godx-platform-framework follows the same rule: point every module at the **same URL**, give each module its **own prefix**.

| Module | Env prefix | Example key on wire |
|--------|------------|---------------------|
| `cache` | `CACHE_PREFIX` + optional `CACHE_STORE_<NAME>_PREFIX` | `myapp:cache:primary:user:42` |
| `ratelimit` | `RATELIMIT_PREFIX` + optional `RATELIMIT_LIMITER_<NAME>_PREFIX` | `myapp:ratelimit:api:192.168.1.1` |
| `queue` (redis driver, future) | `QUEUE_*_PREFIX` | `myapp:queue:default:job-id` |

### Production `.env` sketch (single Redis)

```bash
# ── one Redis box ─────────────────────────────────────────────
REDIS_URL=redis://:secret@redis.internal:6379/0

# ── cache ───────────────────────────────────────────────────
CACHE_DEFAULT_STORE=primary
CACHE_STORES=primary
CACHE_PREFIX=myapp:
CACHE_STORE_PRIMARY_DRIVER=redis
CACHE_STORE_PRIMARY_URL=${REDIS_URL}
CACHE_STORE_PRIMARY_PREFIX=cache:

# ── rate limit (shared across all API pods) ─────────────────
RATELIMIT_DEFAULT=api
RATELIMIT_LIMITERS=api
RATELIMIT_PREFIX=myapp:
RATELIMIT_LIMITER_API_DRIVER=redis
RATELIMIT_LIMITER_API_URL=${REDIS_URL}
RATELIMIT_LIMITER_API_PREFIX=ratelimit:
RATELIMIT_LIMITER_API_RATE=100
RATELIMIT_LIMITER_API_BURST=20
```

Blank-import both heavy drivers in `main.go`:

```go
import (
    _ "github.com/godx-jp/godx-platform-framework/cache/drivers/redis"
    _ "github.com/godx-jp/godx-platform-framework/ratelimit/drivers/redis"
)
```

### What is *not* shared (and why that's OK)

Each module opens its **own** `go-redis` client pool to the URL you configure. That is slightly different from Laravel's single `RedisManager` singleton, but operationally identical:

- Same host, same port, same password — **one Redis server to operate**
- Prefixes prevent cross-module key clashes
- Connection count ≈ `(modules using redis) × (pool size)` — tune pool sizes if you run many modules on one pod

Laravel also opens multiple connections when you use `Redis::connection('cache')` vs `Redis::connection('default')`; the framework's per-module client is the same trade-off.

### When to use a separate Redis DB index

Use `/0` for cache and `/1` for ratelimit **only** if your ops team prefers DB-level isolation. Prefixes alone are enough for most teams (and match Laravel's default).

```bash
CACHE_STORE_PRIMARY_URL=redis://:secret@host:6379/0
RATELIMIT_LIMITER_API_URL=redis://:secret@host:6379/1
```

`Flush()` on cache only touches keys under the cache prefix (SCAN + UNLINK), so ratelimit keys on the same DB stay safe.

---

## Driver pattern recap

| Kind | Registration | Examples |
|------|--------------|----------|
| **Light** | auto on `import ".../module"` | cache `memory`, `file`; ratelimit `memory`; mail `log`, `smtp` |
| **Heavy** | blank import by consumer | cache `redis`; ratelimit `redis`; mail `ses`; secrets `vault` |

Heavy drivers stay out of your binary until you opt in — a CLI tool that only needs in-memory cache never pulls `go-redis`.

---

## Module composition — typical API service

```go
app := framework.New("billing-api", "1.0.0").
    Use(observability.Module).
    Use(config.Module).
    Use(cache.Module).
    Use(ratelimit.Module).
    Use(validation.Module).      // Validator on App
    Use(httpclient.Module).
    Use(events.Module).
    Use(httpx.Module)              // chi + middleware stack
```

### Use case: public REST API with rate limit + cache

1. **Read-heavy endpoint** — cache JSON from DB for 5 minutes (`cache.Store.PutJSON`).
2. **Write endpoint** — validate DTO (`validation.ValidateStruct`), forget cache key on success.
3. **All routes** — wrap router with `ratelimit/middleware.ByIP` (100 req/min per IP).
4. **Outbound webhooks** — `httpclient` resilient driver with OTel spans.

See [modules/httpx.md](./modules/httpx.md) for wiring validation + ratelimit in one middleware stack.

### Use case: multi-tenant SaaS

- `CACHE_PREFIX=tenant-${TENANT_ID}:` via programmatic config per request **or** separate store names per tenant tier.
- `RATELIMIT_LIMITER_API_PREFIX=tenant-${TENANT_ID}:` for per-tenant API quotas.
- Secrets per tenant: `secrets` file driver with `SECRETS_FILE_PATH=/mnt/secrets/${TENANT_ID}`.

### Use case: local dev without Redis

```bash
CACHE_STORES=memory
RATELIMIT_LIMITERS=memory
```

Same code paths; swap to Redis URLs in staging/production only.

---

## Testing strategy (project-wide)

Every module ships:

| Layer | File pattern | Purpose |
|-------|--------------|---------|
| Driver registry | `driver/registry_test.go` | Register / Lookup / New / panic on bad input |
| Conformance | `conformance_test.go` | Driver-agnostic behaviour (memory, file, env, …) |
| Edge cases | `edges_test.go` | Concurrency, shutdown, nil rejection |
| Module wiring | `module_test.go` | `framework.App` init, env config, duplicate init |
| Per-driver | `drivers/*/*_test.go` | Round-trip, driver-specific semantics |
| Integration | `//go:build integration` | Optional Docker (Redis, MailHog) — run manually in CI |

Run everything:

```bash
go test -race ./...
```

Redis integration (cache example):

```bash
docker run --rm -d -p 6379:6379 redis:7-alpine
go test -tags integration ./cache/drivers/redis/ -v
```

---

## Documentation map

| Doc | Content depth |
|-----|----------------|
| [modules/cache.md](./modules/cache.md) | Full — API table, JSON helpers, Redis flush semantics |
| [modules/ratelimit.md](./modules/ratelimit.md) | Full — middleware, shared Redis, use cases |
| [modules/validation.md](./modules/validation.md) | Full — rule table, i18n, nested structs |
| [modules/secrets.md](./modules/secrets.md) | Full — driver matrix, key normalisation |
| [modules/httpclient.md](./modules/httpclient.md) | Full — drivers, resilient, OTel |
| [CONFIGURATION.md](./CONFIGURATION.md) | Every env var |
| [DRIVER_PATTERN.md](./DRIVER_PATTERN.md) | Light vs heavy, lifecycle |

Modules added in the v0.9–v1.0 wave follow the same bar; expand any gap by opening an issue — API is frozen at v1.0.0.

---

## Laravel ↔ framework quick reference

| Laravel | godx-platform-framework |
|---------|-------------------------|
| `config/database.php` redis default | Same `REDIS_URL`, per-module prefix env vars |
| `Cache::store('redis')` | `cache.Manager.Store("primary")` |
| `RateLimiter::for('api', …)` | `ratelimit.Manager` + `middleware.ByIP` |
| `Http::retry(3, 100)` | `httpclient` resilient driver |
| `Validator::make($data, $rules)` | `validation.ValidateStruct` |
| `Mail::to(...)->send()` | `mail.Mailer().To(...).Send()` |
| `Route::middleware('throttle:api')` | `httpx` + `ratelimit/middleware` |
