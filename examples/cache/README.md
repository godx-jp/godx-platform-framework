# cache example

End-to-end demo of the `cache` module. Walks through `Put` / `Get`, `Remember`, atomic `Increment`, `PutJSON` / `GetJSON`, and `Pull` against the configured default store.

## Variants

### In-memory (zero config)

```bash
go run ./examples/cache
```

Default store is `memory` — no setup, no on-disk artefacts.

### File-backed (Laravel-faithful)

```bash
CACHE_DEFAULT_STORE=files \
CACHE_STORES=files \
CACHE_STORE_FILES_DRIVER=file \
CACHE_STORE_FILES_PATH=./.tmp/cache-example \
go run ./examples/cache
```

Files live under `./.tmp/cache-example/<XX>/<YY>/<sha1>.cache` — sharded two levels deep to keep directories browsable on every filesystem.

### Redis

```bash
docker run --rm -d --name redis -p 6379:6379 redis:7
CACHE_DEFAULT_STORE=redis \
CACHE_STORES=redis \
CACHE_STORE_REDIS_URL=redis://127.0.0.1:6379/0 \
CACHE_PREFIX=example: \
go run ./examples/cache
```

The redis driver is heavy — `examples/cache/main.go` already pulls it in via a blank import. Production binaries that don't use redis should omit that import to keep their go.sum lean.

## What the example prints

```
default cache store = memory (registered: [memory])
greeting = hello, world
computing expensive value (this prints only on a cache miss)
expensive (1st call) = 42
expensive (2nd call) = 42        ← cache hit; no recompute
visits after incr #1 = 1
visits after incr #2 = 2
visits after incr #3 = 3
weather = {City:Tokyo Temp:23.7}
pulled flash = one-shot
flash is gone after pull — as expected
```
