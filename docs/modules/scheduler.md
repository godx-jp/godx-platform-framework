# Scheduler

> Cron-based job scheduling with overlap protection, distributed locks, and Laravel-style filters.

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

## Schedule API

| Method | Description |
|--------|-------------|
| `EveryMinute()` | `@every 1m` |
| `Hourly()` / `DailyAt("HH:MM")` / `WeeklyOn(weekday, "HH:MM")` | Cron helpers (six-field, seconds) |
| `Cron(expr)` | robfig/cron expression |
| `WithoutOverlapping()` | In-process mutex — skip when previous run still active |
| `OnOneServer()` | Distributed lock — skip when another replica holds the lock |
| `Timeout(d)` | Per-run context deadline |
| `Between("08:00","17:00")` | Daily time window |
| `Environments("production", …)` | Match `APP_ENV` |
| `When(fn)` / `Unless(fn)` | Conditional skip |
| `RunOnQueue("jobs")` | Push job name to queue via `Options.QueuePush` |

## Lock adapters

| Adapter | Package | Use case |
|---------|---------|----------|
| Memory | `scheduler/lock.Memory` | `WithoutOverlapping` (auto-created) |
| Cache | `scheduler/lock.Cache` | `OnOneServer` via `lock.CacheStore` (TTL renewal when store implements `RenewableStore`) |
| Redis | `scheduler/lock.Redis` / `RedisStore` | `OnOneServer` without cache module |

```go
import "github.com/godx-jp/godx-platform-framework/cache"

mgr, _ := cache.FromApp(app)
app.Use(scheduler.ModuleWithConfig(cfg, mgr.Default()))
```

## Observability

Lifecycle events on `events.Bus`: `schedule.started`, `schedule.finished`, `schedule.failed`, `schedule.skipped`.

`sched.LastRun(name)` and `sched.Health()` expose last status per registered job.

`scheduler.SetMaintenanceMode(true)` skips all runs (Laravel `down` equivalent).

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
