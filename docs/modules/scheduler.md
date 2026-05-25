# Scheduler

> Cron-based job scheduling with overlap protection and distributed locks.

## Quick start

```go
app := framework.New("svc", "1.0.0").Use(scheduler.Module)
_ = app.Init(ctx)
sched, _ := scheduler.FromApp(app)

sched.EveryMinute().WithoutOverlapping().Do("heartbeat", func(ctx context.Context) error {
    return ping(ctx)
})
sched.Cron("0 2 * * *").OnOneServer().Do("nightly", nightly)
```

## Schedule API

| Method | Description |
|--------|-------------|
| `EveryMinute()` | Standard cron `*/1 * * * *` |
| `Cron(expr)` | Five-field cron expression (robfig/cron/v3) |
| `WithoutOverlapping()` | In-process mutex — skip when previous run still active |
| `OnOneServer()` | Distributed lock via `cache.Store` Add semantics |

## Lock adapters

| Adapter | Package | Use case |
|---------|---------|----------|
| Memory | `scheduler/lock.Memory` | `WithoutOverlapping` (auto-created) |
| Cache | `scheduler/lock.Cache` | `OnOneServer` — pass `cache.Store` implementing `lock.CacheStore` |

```go
import "github.com/godx-jp/godx-platform-framework/cache"

mgr, _ := cache.FromApp(app)
app.Use(scheduler.ModuleWithConfig(cfg, mgr.Default()))
```

## Env vars

| Variable | Default | Purpose |
|----------|---------|---------|
| `SCHEDULER_ENABLED` | `true` | Start cron on module Init |
| `SCHEDULER_LOCK_TTL` | `24h` | OnOneServer lock TTL |
| `SCHEDULER_LOCK_PREFIX` | `schedule-lock:` | Cache key prefix |

## Laravel mapping

| Laravel | Framework |
|---------|-----------|
| `$schedule->everyMinute()` | `sched.EveryMinute().Do(...)` |
| `$schedule->cron('0 2 * * *')` | `sched.Cron("0 2 * * *").Do(...)` |
| `->withoutOverlapping()` | `.WithoutOverlapping()` |
| `->onOneServer()` | `.OnOneServer()` + cache lock |
