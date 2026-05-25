# Scheduler

> Cron-based job scheduling with overlap protection, distributed locks,
> per-job health, lifecycle events, and Laravel-style filters. Built on
> `robfig/cron/v3` and depends on the `events` module for hooks.

## Concepts

A `Scheduler` owns a `cron.Cron` runner and a slice of job definitions.
Each call to a schedule-builder (`EveryMinute`, `Cron`, `DailyAt`, …)
returns a fluent `*Schedule`; chaining filters and ending in `Do(name, fn)`
registers the job. At fire time `runJob` evaluates filters, optionally
takes a lock, runs the callback under an optional timeout, records health,
and emits events.

```
Scheduler (cron runner)
   ├─ Schedule (fluent builder)  ── filters ── Do(name, fn)
   ├─ memLock  (lock.Memory)     ── WithoutOverlapping
   ├─ distLock (lock.Mutex)      ── OnOneServer
   ├─ health   map[name]JobHealth
   └─ events.Bus: schedule.started / finished / failed / skipped
```

## Quick start

```go
app := framework.New("svc", "1.0.0").Use(scheduler.Module)
_ = app.Init(ctx)
sched, _ := scheduler.FromApp(app)

sched.EveryMinute().WithoutOverlapping().Do("heartbeat", func(ctx context.Context) error {
    return ping(ctx)
})
sched.DailyAt("02:00").OnOneServer().Do("nightly", nightly)
sched.WeeklyOn(time.Monday, "09:00").Environments("production").Do("report", report)
```

`scheduler.Module` starts the cron runner during `Init` (unless
`SCHEDULER_ENABLED=false`). Register every job **before** `Start` —
`Do` returns an error if called after the scheduler has started.

## Standalone construction

`scheduler.New(Options)` builds a scheduler without the framework:

```go
sched := scheduler.New(scheduler.Options{
    Location:        time.UTC,        // default UTC
    DefaultTimeout:  30 * time.Second, // applied when a job sets no Timeout
    Bus:             bus,             // events.Bus, optional
    DistributedLock: distLock,        // lock.Mutex for OnOneServer, optional
    QueuePush:       push,            // QueuePushFunc for RunOnQueue, optional
    OnRun:           func(job string, err error) { /* per-run callback */ },
})
_ = sched.Start(ctx)
defer sched.Stop(ctx)
```

`Options` fields are all optional; `WithDistributedLock(m)` and
`WithQueuePush(fn)` set the lock and queue-push callback after
construction.

## Schedule-builder API

Builders on `*Scheduler` open a fluent `*Schedule`; filters chain and
`Do(name, fn)` registers the job.

| Builder | Cron produced |
|--------|----------------|
| `EveryMinute()` | `@every 1m` |
| `Hourly()` | `0 0 * * * *` |
| `DailyAt("HH:MM")` | `0 M H * * *` (falls back to midnight on parse error) |
| `WeeklyOn(weekday, "HH:MM")` | `0 M H * * D` (0 = Sunday) |
| `Cron(expr)` | Raw six-field expression (seconds, minutes, hours, dom, month, dow) plus `@descriptor` forms — the parser is configured with the seconds field |

Filters on `*Schedule`:

| Filter | Description |
|--------|-------------|
| `WithoutOverlapping()` | In-process mutex — skip when the previous run is still active |
| `OnOneServer()` | Distributed lock — skip when another replica holds the lock (skips with `lock_misconfigured` when no lock is wired) |
| `Timeout(d)` | Per-run context deadline (falls back to `Options.DefaultTimeout`) |
| `Between("08:00","17:00")` | Daily time window (24h `HH:MM`; supports windows crossing midnight) |
| `Environments("production", …)` | Run only when `APP_ENV` matches |
| `When(fn)` | Skip when `fn()` returns false |
| `Unless(fn)` | Skip when `fn()` returns true |
| `RunOnQueue("jobs")` | Dispatch the job **name** to a queue via `Options.QueuePush` instead of running `fn` inline (`fn` may be nil) |
| `Do(name, fn) error` | Register; errors on empty name, nil callback (without `RunOnQueue`), invalid cron, or registration after `Start` |

Inspection: `sched.Jobs() []string` returns registered job names in
registration order.

## Lock subpackage

`scheduler/lock` defines the locking contract and three adapters. The
core interface:

```go
type Mutex interface {
    TryAcquire(ctx context.Context, key string) (release func() error, ok bool, err error)
}
```

`WithoutOverlapping` always uses an auto-created `lock.Memory`.
`OnOneServer` uses whatever `Mutex` was supplied via `Options.DistributedLock`
or `WithDistributedLock`.

| Adapter | Constructor | Use case |
|---------|-------------|----------|
| `lock.Memory` | `lock.NewMemory()` | Process-local mutex map for `WithoutOverlapping` (auto-created by `New`) |
| `lock.Cache` | `lock.NewCache(lock.CacheOptions{Store, Prefix, Owner, TTL})` | Distributed lock over a `lock.CacheStore` (cache module `Store` satisfies it) |
| `lock.Redis` | `lock.NewRedis(lock.RedisOptions{Client, Prefix, Owner, TTL})` | Direct Redis `SET NX EX` lock, no cache module needed |

Supporting interfaces:

```go
// Minimum a cache backend must provide for OnOneServer.
type CacheStore interface {
    Add(ctx, key, value, ttl) (added bool, err error)
    Forget(ctx, key) error
}
// Adds TTL renewal for long-running jobs.
type RenewableStore interface {
    CacheStore
    Renew(ctx, key, value, ttl) error
}
```

`lock.Cache` and `lock.Redis` both run a background renewal loop
(interval = `TTL/3`, floored at 1s) that re-extends the lock while the
job runs; the `release` callback stops the loop and releases the key.
`lock.Cache` only renews when its store implements `RenewableStore`.
`lock.NewRedisStore(client)` adapts a `*redis.Client` to
`CacheStore`/`RenewableStore` so it can be passed to
`scheduler.ModuleWithConfig`.

#### Per-acquisition token (ownership safety)

Every `TryAcquire` call stores a **unique per-acquisition token**
(`Owner` + a `crypto/rand` random suffix), not the bare configured
`Owner`. Two replicas that share the same `Owner` therefore never share
an effective lock value. This makes release and renewal **owner-checked**:

- **`lock.Redis` release** is a compare-and-delete Lua script
  (`if GET(key) == token then DEL(key)`), so a replica only ever deletes
  the lock it actually holds. If replica A's lock expires under load and
  replica B acquires it, A's release no longer deletes B's lock — closing
  the lock-theft / accidental-release race that previously let an
  `OnOneServer` job run concurrently.
- **Renewal** (both adapters) is value-checked against the same captured
  token, so a holder that has already lost the lock can never re-extend
  or resurrect one another replica now owns.
- **`RedisStore.Forget`** (the `CacheStore` adapter path used by
  `scheduler.ModuleWithConfig`) remains an unconditional `DEL`, because
  the `CacheStore.Forget(ctx, key)` contract — shared with the cache
  module's `Store` — carries no token argument. The residual race on that
  path is bounded by value-checked `Renew` plus the per-acquisition token;
  callers needing strict compare-and-delete on release should use the
  direct `lock.Redis` lock (`NewRedis`).

#### Renewal-failure behavior

When a renewal returns an error or a result indicating the key was lost
(expired or taken over by another replica), the lock no longer holds even
though the job is still running, so the `OnOneServer` guarantee is
temporarily broken. Both adapters now **log a warning via `log/slog`**
(rather than silently discarding the result) so operators can detect and
alert on it. The in-flight job is **not** cancelled — wiring a cancel
signal through `runJob` is a deliberate follow-up; the current behavior is
detect-and-log, documented inline with `// SECURITY:` comments in the
lock code.

### Wiring `OnOneServer` via the module

```go
import "github.com/godx-jp/godx-platform-framework/cache"

mgr, _ := cache.FromApp(app)
app.Use(scheduler.ModuleWithConfig(cfg, mgr.Default()))
```

`ModuleWithConfig(cfg, cacheLock)` takes an optional `lock.CacheStore`;
pass `nil` when `OnOneServer` is unused. When supplied, the module
builds a `lock.Cache` using `cfg.LockPrefix`, `cfg.LockTTL`, and
`app.Name()` as the owner.

## Observability

Lifecycle events on `events.Bus` (constants `scheduler.EventStarted` etc.):

| Event constant | Value | When |
|---|---|---|
| `EventStarted` | `schedule.started` | Just before the callback runs |
| `EventFinished` | `schedule.finished` | Callback returned nil (detail = duration in ms) |
| `EventFailed` | `schedule.failed` | Callback returned an error |
| `EventSkipped` | `schedule.skipped` | A filter/lock prevented the run (detail names the reason: `maintenance`, `environment`, `between`, `when`, `unless`, `lock_misconfigured`, `lock_busy`, `overlap`) |

Event metadata: `job`, optional `detail`, optional `error`.

`sched.LastRun(name) (time.Time, bool)` and `sched.Health() map[string]JobHealth`
expose the last run timestamp, status, and detail per job
(`JobHealth{Name, LastRun, LastStatus, LastDetail}`).

`scheduler.SetMaintenanceMode(true)` skips all runs (Laravel `down`
equivalent); `scheduler.MaintenanceMode()` reports the current state.
`scheduler.CurrentEnvironment()` returns `APP_ENV` or `"production"`.

## Env vars

| Variable | Default | Purpose |
|----------|---------|---------|
| `SCHEDULER_ENABLED` | `true` | Start cron on module Init |
| `SCHEDULER_LOCK_TTL` | `24h` | OnOneServer lock TTL |
| `SCHEDULER_LOCK_PREFIX` | `schedule-lock:` | Cache key prefix |
| `APP_ENV` | `production` | Used by `Environments()` |

## Laravel mapping

| Laravel | Framework |
|---------|-----------|
| `$schedule->everyMinute()` | `sched.EveryMinute().Do(...)` |
| `$schedule->dailyAt('02:00')` | `sched.DailyAt("02:00").Do(...)` |
| `$schedule->weeklyOn(1, '9:00')` | `sched.WeeklyOn(time.Monday, "09:00").Do(...)` |
| `->withoutOverlapping()` | `.WithoutOverlapping()` |
| `->onOneServer()` | `.OnOneServer()` + cache/redis lock |
| `->environments('production')` | `.Environments("production")` |
| `->between('8:00','17:00')` | `.Between("08:00","17:00")` |
| `->when(fn)` / `->unless(fn)` | `.When(fn)` / `.Unless(fn)` |
| `->runInBackground()` | `.RunOnQueue("default")` + `QueuePush` |

## Error model

There are no sentinel errors. `Do` returns a descriptive error for
invalid registration (empty name, nil callback without `RunOnQueue`,
invalid cron expression, or registration after `Start`). A callback that
returns an error emits `schedule.failed` and records failed health, but
does not stop the runner — subsequent fires proceed. `Start` is
idempotent and returns nil if already started; `Stop` is idempotent and
returns nil if not started.

## Context propagation

`scheduler.ContextWithScheduler(ctx, sched)` attaches a scheduler to a
context and `scheduler.FromContext(ctx)` retrieves it.
`scheduler.FromApp(app)` is the canonical way to fetch the scheduler
built by `scheduler.Module`. Job callbacks receive a fresh
`context.Background()` (wrapped with the per-run timeout when set), not
the registration context.

## Lifecycle

`scheduler.Module` stores the `Scheduler` under `scheduler.StoreKey`,
registers `Stop` as an `OnShutdown` callback, and — unless
`SCHEDULER_ENABLED=false` — calls `Start` during `Init`. `Stop` halts
the cron runner and waits for in-flight jobs (bounded by the shutdown
context). Only one `scheduler.Module` may be installed per `App`.
