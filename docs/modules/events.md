# Events

> In-process event bus. Subscribe by exact name or wildcard pattern;
> Dispatch fans every event out to every matching listener. Wrap the
> Bus in NewAsync for fire-and-forget worker-pool semantics.

## Concepts

A single `Bus` owns all subscriptions. Subscribers register by pattern; the dispatcher walks every subscription on every `Dispatch` and invokes matching listeners synchronously by default. The async wrapper turns Dispatch into a queue-and-return operation handled by a background worker pool — Laravel's `ShouldQueue` is the spiritual cousin.

```
Bus
 ├─ subscriptions [pattern → Listener]
 ├─ Dispatch(ctx, Event)
 │   └─ match → invoke (sync, errors joined)
 └─ Close (idempotent)

NewAsync(Bus, Options)
 ├─ queue (buffered chan)
 ├─ workers × Goroutine
 └─ Close drains pending jobs
```

## Quick start

```go
package main

import (
    "context"
    "log"

    "github.com/godx-jp/godx-platform-framework/events"
    "github.com/godx-jp/godx-platform-framework/framework"
)

func main() {
    ctx := context.Background()
    app := framework.New("svc", "1.0.0").Use(events.Module)
    if err := app.Init(ctx); err != nil { log.Fatal(err) }
    defer app.Shutdown(ctx)

    bus, _ := events.FromApp(app)
    bus.Listen("user.created", func(ctx context.Context, e events.Event) error {
        log.Printf("welcome email for %v", e.Payload)
        return nil
    })
    bus.Listen("user.*", auditListener)        // wildcard
    _ = bus.Dispatch(ctx, events.Event{Name: "user.created", Payload: "alice"})
}
```

## Wildcard patterns

| Pattern | Matches |
|---|---|
| `*` | every event |
| `user.*` | every `user.<anything>` event (multi-segment trailing) |
| `*.deleted` | every `<anything>.deleted` event |
| `user.*.email` | `user.<one-segment>.email` |
| `user.created` | exact match only |

`*` inside a name matches one or more dot-separated segments when it is the trailing or leading wildcard; in the middle it matches exactly one segment.

## Async dispatch

```go
bus := events.NewAsync(events.New(), events.AsyncOptions{
    Workers:   4,
    QueueSize: 1024,
    OnError: func(err error) {
        log.Printf("event listener failed: %v", err)
    },
})
defer bus.Close(ctx)
```

Or wire it via the module:

```bash
EVENTS_ASYNC=true
EVENTS_ASYNC_WORKERS=4
EVENTS_ASYNC_QUEUE_SIZE=1024
```

`Close(ctx)` drains the queue before returning — workers process every job already queued, then exit. If `ctx` expires first, the inner Bus is still closed but pending jobs may be lost.

## Listener errors

The synchronous Bus returns a joined error containing every listener's error — siblings still run even when one fails. The async wrapper surfaces them via `Options.OnError` instead.

Panicking listeners are caught; the panic value is converted to an error so siblings remain protected.

## Subscription handle

```go
s := bus.Listen("orders.*", h)
// later, in the same goroutine or another:
s.Cancel()
```

Subscriptions are also bulk-cancellable by pattern:

```go
removed := bus.Forget("orders.*")
```

## Env var reference

| Var | Purpose | Default |
|---|---|---|
| `EVENTS_ASYNC` | Wrap the in-process bus with `NewAsync` | `false` |
| `EVENTS_ASYNC_WORKERS` | Worker-pool size when async | `4` |
| `EVENTS_ASYNC_QUEUE_SIZE` | Buffered queue depth when async | `256` |

## Laravel API mapping

| Laravel | Framework |
|---|---|
| `Event::listen('user.created', $h)` | `bus.Listen("user.created", h)` |
| `Event::listen('user.*', $h)` | `bus.Listen("user.*", h)` |
| `Event::dispatch(new UserCreated($u))` | `bus.Dispatch(ctx, events.Event{Name: "user.created", Payload: u})` |
| `Event::forget('user.created')` | `bus.Forget("user.created")` |
| `Event::until(...)` (halt-on-non-nil) | not provided — Listener errors are joined, not short-circuiting |
| `Subscriber` (class-based) | use a struct with a `Register(bus)` method that registers every pattern |
| `ShouldQueue` queue listeners | wrap the Bus in `events.NewAsync(...)` |

## Migrating from go-common

`umbrella/packages/go-common` does not ship a standalone event bus; teams roll their own callbacks or use the outbox pattern directly. The events module gives those callbacks a single place to register and a uniform context-aware contract.

| Before | After |
|---|---|
| `cb := func(...)` field on a service | `bus.Listen("user.created", cb)` |
| Per-service notify slices | One Bus on the App, register at Module init |
| `go func() { ... }()` lifecycle hooks | `events.NewAsync(bus, AsyncOptions{...})` plus listener |
| Outbox publisher | Listen to `*` and forward to the outbox writer |

## Out of scope

- **Persistent queue** — handled by the upcoming `queue` module (`v0.11.x`). The async wrapper is in-memory only.
- **Schema validation** — handled by the upcoming `validation` module (`v0.9.0`).
- **Dead-letter** — the async wrapper passes errors to `OnError`; durable retry is a queue concern.
