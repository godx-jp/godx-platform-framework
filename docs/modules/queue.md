# Queue

> Laravel-style job queues with swappable backends and events-module lifecycle hooks.

## Concepts

```
Manager
 └─ Queue (named connection)
     ├─ Push(ctx, queue, payload)
     ├─ Dispatch(ctx, queue, handler)   — process one job
     ├─ Run(ctx, queue)                  — background workers
     └─ events: job.processing / job.processed / job.failed
```

## Quick start

```go
import (
    "github.com/godx-jp/godx-platform-framework/framework"
    "github.com/godx-jp/godx-platform-framework/queue"
)

app := framework.New("svc", "1.0.0").Use(queue.Module)
_ = app.Init(ctx)

mgr, _ := queue.FromApp(app)
q, _ := mgr.Default()
_, _ = q.Push(ctx, "emails", []byte(`{"to":"user@example.com"}`))
```

## Drivers

| Driver | Env | Registration | Status |
|--------|-----|--------------|--------|
| memory | `QUEUE_QUEUE_<NAME>_DRIVER=memory` | auto | stable |
| sqs | `QUEUE_QUEUE_<NAME>_DRIVER=sqs` | opt-in (`_ ".../queue/drivers/sqs"`) | stub |
| kafka | `QUEUE_QUEUE_<NAME>_DRIVER=kafka` | opt-in | stub |
| nats | `QUEUE_QUEUE_<NAME>_DRIVER=nats` | opt-in | stub |

## Environment

| Variable | Default | Description |
|----------|---------|-------------|
| `QUEUE_DEFAULT` | `default` | Default connection name |
| `QUEUE_QUEUES` | `default` | Comma-separated connection names |
| `QUEUE_QUEUE_<NAME>_DRIVER` | `memory` | Backend driver |
| `QUEUE_QUEUE_<NAME>_DEFAULT` | `default` | Default queue name on connection |
| `QUEUE_QUEUE_<NAME>_WORKERS` | `1` | Worker count for Run |
| `QUEUE_QUEUE_<NAME>_SIZE` | `256` | Memory driver channel capacity |

## Events

Wire `events.Module` and pass the bus via `queue.ModuleWithConfig(cfg, bus)` or `queue.WithBus`:

| Event | When |
|-------|------|
| `job.processing` | Before handler runs |
| `job.processed` | After successful delete |
| `job.failed` | Handler returned error (job released for retry) |

Metadata keys: `queue`, `job_id`, `attempts`, `error` (on failure).

See [examples/queue/main.go](../../examples/queue/main.go).
