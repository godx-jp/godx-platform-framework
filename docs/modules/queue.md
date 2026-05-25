# Queue

> Laravel-style job queues with swappable backends, automatic retry with
> exponential backoff, dead-letter routing, and events-module lifecycle
> hooks. Pick a backend per environment via configuration; application
> code talks to one `Queue` handle and never imports a driver.

## Concepts

A `Manager` owns one or more named `Queue` connections; each connection
wraps a `driver.Backend` chosen at deploy time (`memory`, `redis`, …).
A `Queue` pushes raw `[]byte` payloads, dispatches them to a
`driver.Handler`, and on failure retries with exponential backoff before
routing to a dead-letter queue (DLQ).

```
Manager ── named Queues
   └─ Queue(name)
        ├─ Push(ctx, queue, payload)        — enqueue
        ├─ Dispatch(ctx, queue, handler)    — process one job
        ├─ Run(ctx, queue)                  — background workers
        └─ driver.Backend (memory · redis · sqs/kafka/nats stubs)
              events: job.processing / job.processed / job.failed / job.dead
```

## Quick start

```go
package main

import (
    "context"

    "github.com/godx-jp/godx-platform-framework/framework"
    "github.com/godx-jp/godx-platform-framework/queue"
)

func main() {
    ctx := context.Background()
    app := framework.New("svc", "1.0.0").Use(queue.Module)
    if err := app.Init(ctx); err != nil { panic(err) }
    defer app.Shutdown(ctx)

    mgr, _ := queue.FromApp(app)
    q, _ := mgr.Default()
    _, _ = q.Push(ctx, "emails", []byte(`{"to":"user@example.com"}`))
}
```

With nothing in the environment you get a single in-memory connection
named `default` using the `memory` driver.

## Configuration

### Env vars

```bash
QUEUE_DEFAULT=primary
QUEUE_QUEUES=primary,emails

QUEUE_QUEUE_PRIMARY_DRIVER=memory
QUEUE_QUEUE_PRIMARY_WORKERS=4

QUEUE_QUEUE_EMAILS_DRIVER=redis
QUEUE_QUEUE_EMAILS_REDIS_URL=redis://:secret@127.0.0.1:6379/0
QUEUE_QUEUE_EMAILS_REDIS_PREFIX=emails:queue:
```

| Variable | Default | Description |
|----------|---------|-------------|
| `QUEUE_DEFAULT` | `default` | Default connection name (must be in `QUEUE_QUEUES`) |
| `QUEUE_QUEUES` | `default` | Comma-separated connection names |
| `QUEUE_QUEUE_<NAME>_DRIVER` | `memory` | Backend driver |
| `QUEUE_QUEUE_<NAME>_DEFAULT` | `default` | Default queue name on the connection (used when `Push`/`Dispatch` get `""`) |
| `QUEUE_QUEUE_<NAME>_WORKERS` | `1` | Worker goroutine count for `Run` |
| `QUEUE_QUEUE_<NAME>_SIZE` | `256` | Memory driver channel capacity |
| `QUEUE_QUEUE_<NAME>_AWS_REGION` | `$AWS_REGION` | SQS region |
| `QUEUE_QUEUE_<NAME>_URL` | — | SQS queue URL |
| `QUEUE_QUEUE_<NAME>_BROKERS` | — | Kafka brokers (CSV) |
| `QUEUE_QUEUE_<NAME>_TOPIC` | — | Kafka topic |
| `QUEUE_QUEUE_<NAME>_GROUP` | — | Kafka consumer group |
| `QUEUE_QUEUE_<NAME>_NATS_URL` | `$NATS_URL` | NATS server URL |
| `QUEUE_QUEUE_<NAME>_SUBJECT` | — | NATS subject |
| `QUEUE_QUEUE_<NAME>_STREAM` | — | NATS JetStream stream name |
| `QUEUE_QUEUE_<NAME>_REDIS_URL` | `$REDIS_URL` | Redis URL (`redis://…`) |
| `QUEUE_QUEUE_<NAME>_REDIS_ADDRESS` | `$REDIS_ADDRESS` | Redis host:port when URL unset |
| `QUEUE_QUEUE_<NAME>_REDIS_PREFIX` | `godx:queue:` | Redis key prefix |

`Config.Validate` requires at least one queue, a non-empty
`QUEUE_DEFAULT`, and that the default name appears in `QUEUE_QUEUES`.

### Programmatic config

The events bus is wired through `ModuleWithConfig`, not the env module:

```go
cfg := queue.Config{
    Default: "primary",
    Queues: map[string]queue.QueueConfig{
        "primary": {Driver: "memory", Workers: 4},
    },
}
app := framework.New("svc", "1.0.0").
    Use(events.Module).
    Use(queue.ModuleWithConfig(cfg, bus)) // bus is an events.Bus, or nil
```

## Queue API

`Push` and `Dispatch` take the per-call queue name; pass `""` to use the
connection's configured default queue. Payloads are opaque `[]byte`.

| Method | Description | Laravel parallel |
|---|---|---|
| `q.Push(ctx, queue, payload) (driver.Job, error)` | Enqueue payload | `Queue::push` |
| `q.Dispatch(ctx, queue, handler) error` | Pop one job, run handler, emit events, retry/DLQ on failure | `Worker` single tick |
| `q.Run(ctx, queue) error` | Start `WithWorkers(n)` goroutines that loop `Dispatch` until `ctx` is canceled or `Stop` (backs off on backend errors — see below) | `queue:work` |
| `q.Stop()` | Signal `Run` workers to exit and wait for them | — |
| `q.Shutdown(ctx) error` | `Stop` then close the backend | — |
| `q.Name() string` / `q.Backend() driver.Backend` | Accessors | — |

When `Dispatch` receives a `nil` handler it falls back to the handler set
via `WithHandler`; if neither exists it returns an error. `Run` always
uses the default handler, so a `Run`-based worker must be constructed
with `WithHandler`.

#### Worker loop & backend backoff

Each `Run` worker pops with a 100 ms timeout, so an idle (empty) queue is
paced by that timeout — workers do not spin. After a job is processed the
worker loops immediately to stay responsive under load.

If a pop fails with a **real backend error** (e.g. Redis unreachable),
the worker would otherwise retry instantly, busy-spinning a CPU and
hammering the failing backend. Instead it sleeps an **exponential backoff**
before the next attempt: starting at 100 ms, doubling per consecutive
error, capped at 5 s, and reset to the base on the next successful (or
empty) cycle. The empty-queue pop timeout and the error backoff are never
applied together, so a healthy idle queue is not slowed.

The backoff sleep is **interruptible** by both `ctx` cancellation and
`Stop()`, so workers still exit promptly even while waiting out a failing
backend.

### Construction options

`NewQueue(name, backend, opts...)` builds a `Queue` directly (prefer
`queue.Module` for the normal boot path). Options:

| Option | Effect |
|---|---|
| `WithBus(bus events.Bus)` | Emit lifecycle events |
| `WithHandler(h driver.Handler)` | Default handler for `Dispatch`/`Run` |
| `WithDefaultQueue(name string)` | Queue name used when `Push`/`Dispatch` get `""` |
| `WithRetryPolicy(p RetryPolicy)` | Retry/backoff/DLQ tuning |
| `WithWorkers(n int)` | Worker count for `Run` (must be `> 0`) |

### Manager API

`mgr.Default() (*Queue, error)`, `mgr.Queue(name) (*Queue, error)`,
`mgr.Queues() []string`, `mgr.AddQueue(*Queue) error`,
`mgr.SetDefault(name) error`, `mgr.Shutdown(ctx) error`.

## Driver matrix

| Driver | Status | Registration | Notes |
|---|---|---|---|
| `memory` | stable | auto | Buffered channel per queue (`SIZE` capacity), in-process. Light. Ideal for tests and single-process services |
| `redis` | stable | opt-in (`_ "...queue/drivers/redis"`) | `go-redis/v9` backend. Heavy |
| `sqs` | stub | opt-in | Registers the name; constructor returns `driver.ErrNotImplemented` |
| `kafka` | stub | opt-in | Registers the name; constructor returns `driver.ErrNotImplemented` |
| `nats` | stub | opt-in | Registers the name; constructor returns `driver.ErrNotImplemented` |

**Light** drivers (`memory`) auto-register on `import "...queue"`.

**Heavy** drivers (and the stubs) register only on explicit blank import:

```go
import _ "github.com/godx-jp/godx-platform-framework/queue/drivers/redis"
```

Selecting a driver whose package was never imported fails at module init
with a hint naming the missing import path. Out-of-tree drivers register
via `driver.Register(name, constructor)`.

### Backend contract

Every driver implements `driver.Backend`:

```go
type Backend interface {
    Name() string
    Push(ctx context.Context, queue string, payload []byte) (Job, error)
    Pop(ctx context.Context, queue string) (Job, error)
    Delete(ctx context.Context, job Job) error
    Release(ctx context.Context, job Job, delay time.Duration) error
    Shutdown(ctx context.Context) error
}
```

A `Job` exposes `ID()`, `Queue()`, `Payload()`, and `Attempts()`.
`Pop` may return `(nil, nil)` when idle. `Handler` is
`func(ctx context.Context, job Job) error`.

## Retry / DLQ

On a handler error, `Dispatch` consults the connection's `RetryPolicy`:

```go
type RetryPolicy struct {
    MaxAttempts int
    Backoff     BackoffConfig
    DLQSuffix   string
}

type BackoffConfig struct {
    Base time.Duration
    Max  time.Duration
}
```

Defaults (applied per-field when zero): `MaxAttempts: 3`,
`Backoff{Base: 1s, Max: 5m}`, `DLQSuffix: "-dlq"`.

- **Backoff** is **exponential**: attempt *n* waits
  `Base * 2^(n-1)`, capped at `Max`, plus a small random jitter of up to
  10% of the delay. The exponent is clamped at `2^10`.
- When `job.Attempts()+1 >= MaxAttempts`, the job is pushed to
  `<queue><DLQSuffix>` (e.g. `emails-dlq`), deleted from the source
  queue, and `job.dead` is emitted.
- Otherwise the job is `Release`d back to the queue with the computed
  delay and `job.failed` is emitted.

## Events

Wire `events.Module` and pass the bus via `queue.ModuleWithConfig(cfg, bus)`
(or `queue.WithBus` when constructing manually). When no bus is set,
events are silently skipped.

| Event constant | Value | When |
|---|---|---|
| `queue.EventProcessing` | `job.processing` | Before the handler runs |
| `queue.EventProcessed` | `job.processed` | After a successful delete |
| `queue.EventFailed` | `job.failed` | Handler errored, job released for retry |
| `queue.EventDead` | `job.dead` | Max attempts reached, job routed to DLQ |

Metadata keys: `queue`, `job_id`, `attempts`, and `error` (on failure /
dead).

## Error model

| Error | When |
|---|---|
| `driver.ErrNotFound` | Job no longer present in the backend |
| `driver.ErrClosed` | Backend used after `Shutdown` |
| `driver.ErrNotImplemented` | Stub driver (`sqs`/`kafka`/`nats`) constructor |

`Dispatch` returns the handler's error after emitting the appropriate
event. Backend push/pop/delete/release errors propagate directly.

## Context propagation

`queue.ContextWithManager(ctx, mgr)` attaches a manager to a context;
`queue.FromContext(ctx)` retrieves it. `queue.FromApp(app)` is the
canonical way to retrieve the manager built by `queue.Module`.

## Lifecycle

`queue.Module` stores the `Manager` under `queue.StoreKey` and registers
an `OnShutdown` callback that walks every connection and calls
`Shutdown` (which stops `Run` workers and closes the backend). Calling
`mgr.Shutdown` manually is safe.

## Conformance

`queue/conformance_test.go` exercises the shared `driver.Backend`
contract against the memory driver; extend with Redis under integration
CI.

See [examples/queue/main.go](../../examples/queue/main.go).
