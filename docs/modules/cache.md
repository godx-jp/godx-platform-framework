# Cache

> Laravel-faithful multi-store key/value cache. Pick a backend per
> environment via configuration; the application talks to one Laravel-
> style facade and never changes a line.

## Concepts

A single `Manager` holds one or more named `Store` handles; each store is backed by a driver chosen at deploy time (`memory`, `file`, `redis`). Application code never knows or imports a driver — backend swapping is an env-var change. Database-backed cache is intentionally **out of scope** — see [Why no DB driver](#why-no-db-driver).

```
Manager ── named Stores
   └─ Store(name) ── user-facing Laravel-style API
         └─ driver.Driver (memory · file · redis)
```

## Quick start

```go
package main

import (
    "context"
    "time"

    "github.com/godx-jp/godx-platform-framework/cache"
    "github.com/godx-jp/godx-platform-framework/framework"
)

func main() {
    ctx := context.Background()
    app := framework.New("svc", "1.0.0").Use(cache.Module)
    if err := app.Init(ctx); err != nil { panic(err) }
    defer app.Shutdown(ctx)

    mgr, _ := cache.FromApp(app)
    store := mgr.Default()

    _ = store.Put(ctx, "k", []byte("v"), 5*time.Minute)
    v, ok, _ := store.Get(ctx, "k")
    _ = v; _ = ok
}
```

With nothing in the environment you get a single in-memory store named `memory`.

## Multiple stores via env

```bash
CACHE_DEFAULT_STORE=primary
CACHE_STORES=primary,sessions
CACHE_PREFIX=svc:

# primary: redis
CACHE_STORE_PRIMARY_DRIVER=redis
CACHE_STORE_PRIMARY_URL=redis://:secret@127.0.0.1:6379/0
CACHE_STORE_PRIMARY_PREFIX=primary:

# sessions: file
CACHE_STORE_SESSIONS_DRIVER=file
CACHE_STORE_SESSIONS_PATH=./storage/framework/sessions
```

The driver-name shortcut also works — `CACHE_STORES=redis` automatically resolves to `CACHE_STORE_REDIS_DRIVER=redis`, so very small services don't need to repeat the name.

## Programmatic config

For tests or code-driven wiring:

```go
cfg := cache.Config{
    DefaultStore: "primary",
    Stores: map[string]cache.StoreConfig{
        "primary":  {Driver: "memory"},
        "sessions": {Driver: "file", Path: "./var/sessions"},
    },
}
app := framework.New(...).Use(cache.ModuleWithConfig(cfg))
```

Add an extra store after the module has booted:

```go
app.Use(cache.AddStore("audit", cache.StoreConfig{Driver: "memory"}))
```

## Store API

Every method takes `context.Context` first. Errors that aren't a clean miss propagate; `ok == false` means the key is absent or expired.

| Method | Laravel parallel |
|---|---|
| `Get(ctx, key) -> ([]byte, ok, error)` | `Cache::get(key)` |
| `Put(ctx, key, value, ttl) error` | `Cache::put(key, value, ttl)` |
| `Forever(ctx, key, value) error` | `Cache::forever(key, value)` |
| `Add(ctx, key, value, ttl) (bool, error)` | `Cache::add(key, value, ttl)` — write-if-absent |
| `Pull(ctx, key) ([]byte, ok, error)` | `Cache::pull(key)` — read-and-delete |
| `Forget(ctx, key) error` | `Cache::forget(key)` |
| `Has(ctx, key) (bool, error)` / `Missing(ctx, key) (bool, error)` | `Cache::has` / `Cache::missing` |
| `Flush(ctx) error` | `Cache::flush()` — scoped to the store's prefix where supported |
| `Increment(ctx, key, delta) (int64, error)` / `Decrement` | `Cache::increment` / `Cache::decrement` — atomic; counter stored as decimal int |
| `Remember(ctx, key, ttl, fn)` | `Cache::remember(key, ttl, fn)` |
| `RememberForever(ctx, key, fn)` | `Cache::rememberForever(key, fn)` |

### JSON helpers

```go
type Account struct { ID string `json:"id"`; Plan string `json:"plan"` }

_ = store.PutJSON(ctx, "acct:42", Account{ID: "42", Plan: "pro"}, time.Hour)

var a Account
ok, _ := store.GetJSON(ctx, "acct:42", &a)

err := store.RememberJSON(ctx, "acct:42", time.Hour, &a, func(ctx context.Context) (any, error) {
    return fetchAccountFromDB(ctx, "42")
})
```

## Driver matrix

| Driver | Status | Registration | Notes |
|---|---|---|---|
| `memory` | stable | auto | Sync map with periodic TTL sweeper. Light. Ideal for tests and single-process services |
| `file` | stable | auto | One *.cache file per key, JSON envelope `{ exp, val }`. Filenames are SHA-1 of the prefixed key, sharded `<root>/XX/YY/`. Atomic writes via tmp + rename. Light |
| `redis` | stable (v0.7.0) | opt-in (`_ "...cache/drivers/redis"`) | `go-redis/v9` client. Atomic INCRBY/DECRBY; SET NX for Add; SCAN+UNLINK Flush scoped to the configured prefix. Heavy |

**Light** drivers (`memory`, `file`) auto-register on `import "...cache"`.

**Heavy** drivers carry an SDK dependency and register only on explicit blank import, so binaries that only need the in-process stores stay free of the redis dependency:

```go
import _ "github.com/godx-jp/godx-platform-framework/cache/drivers/redis"
```

Selecting `redis` without the blank import fails at module init with a hint that names the missing import path.

## Redis driver

```bash
CACHE_STORES=cache
CACHE_STORE_CACHE_DRIVER=redis
CACHE_STORE_CACHE_URL=redis://:secret@127.0.0.1:6379/0
# or component-wise:
# CACHE_STORE_CACHE_ADDRESS=127.0.0.1:6379
# CACHE_STORE_CACHE_USERNAME=default
# CACHE_STORE_CACHE_PASSWORD=secret
# CACHE_STORE_CACHE_DB=0
# CACHE_STORE_CACHE_TLS=false
# CACHE_STORE_CACHE_PREFIX=cache:        # per-store prefix
```

`Put` maps to `SET` (with `PX` when TTL > 0). `Add` is `SET NX`. `Increment`/`Decrement` use native `INCRBY`/`DECRBY` for full atomicity even under heavy contention. `Flush` walks the namespace with `SCAN` + `UNLINK` when a prefix is set, so multi-tenant Redis instances stay safe — only the keys belonging to this store are removed. Without a prefix, `Flush` falls back to `FLUSHDB` against the selected logical DB.

### Live integration test

```bash
docker run --rm -d --name redis-test -p 6379:6379 redis:7-alpine
go test -tags integration -run TestRedis_Integration -v ./cache/drivers/redis/
docker stop redis-test
```

The test set verifies round-trip, TTL expiry, `SET NX` semantics, atomic counter under 50 concurrent goroutines (must total exactly 500), and prefix-scoped Flush.

## File driver

```bash
CACHE_STORES=disk
CACHE_STORE_DISK_DRIVER=file
CACHE_STORE_DISK_PATH=./storage/framework/cache
```

Each cache file is the JSON `{ "exp": <unix-ms>, "val": <base64> }` envelope. `exp == 0` means store forever. Files are written via a temp file + `os.Rename`, so partial writes never corrupt a key. Per-key locking serialises `Add`/`Increment` within a single process; cross-process safety relies on the rename atomicity of the underlying filesystem.

`Flush` walks the entire `path` and removes every `*.cache` file — prefix scoping is informational only because filenames are hashed. When you need isolated prefixes, give each logical store its own `PATH`.

## Memory driver

```bash
CACHE_STORES=memory
CACHE_STORE_MEMORY_DRIVER=memory
# CACHE_STORE_MEMORY_PREFIX=foo:   # optional
```

In-process `sync.Map` with a background goroutine that sweeps expired entries every 30 s. Values are copied in and out so callers can mutate the returned slice without poisoning the cache.

## Error model

```go
v, ok, err := store.Get(ctx, "k")
switch {
case err != nil:                                  // backend failure
case !ok:                                         // miss
default:                                          // hit
}

// Atomic counter operations expose a typed sentinel when the stored
// value is not a decimal integer:
if _, err := store.Increment(ctx, "name", 1); errors.Is(err, driver.ErrNotInteger) {
    // value at "name" is not numeric — promote/repair it before retrying
}
```

## Context propagation

`cache.ContextWithManager(ctx, mgr)` attaches a manager to a context for handlers that prefer pulling from `context.Context` over a closure. `cache.FromContext(ctx)` retrieves it.

`cache.FromApp(app)` is the canonical way to retrieve the manager built by `cache.Module`.

## Lifecycle

`cache.Module` registers an `OnShutdown` callback on the framework `App` that walks every store and calls `Shutdown` — closing redis pool connections and stopping the memory driver's sweeper. Manual `mgr.Shutdown(ctx)` calls are safe and idempotent.

## Why no DB driver

A database-backed cache table is operationally awkward — every read becomes a SQL round trip, TTL eviction needs a cron job, and the table competes with the application's real workload for locks and buffer cache. `redis` covers low-latency shared caching; `file` covers single-host persistence; `memory` covers within-process caching. If you need a SQL cache for a specific corner case, write a driver in your service and register it via `driver.Register("mydb", construct)` — the module pattern explicitly supports out-of-tree drivers.
