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

## The Event

```go
type Event struct {
    Name      string            // routing key (required) — matched against listener patterns
    Payload   any               // opaque to the bus; handed to listeners as-is
    Metadata  map[string]string // optional out-of-band data
    CreatedAt time.Time         // stamped at dispatch time when left zero
}
```

`Dispatch` returns `events: Event.Name is required` for an empty `Name`, and stamps `CreatedAt` with the current time when the caller leaves it zero.

## Async dispatch

```go
bus := events.NewAsync(events.New(), events.AsyncOptions{
    Workers:   4,    // background workers; <= 0 defaults to 1
    QueueSize: 1024, // buffered queue depth; <= 0 defaults to 256
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

The module's `ModuleWithConfig(cfg events.Config, onError func(error))` passes `onError` through to the async wrapper. Through the module path `EVENTS_ASYNC_WORKERS` and `EVENTS_ASYNC_QUEUE_SIZE` default to `4` and `256`; constructing `NewAsync` directly defaults `Workers` to `1`.

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

Subscriptions are also bulk-cancellable by pattern (`Forget` removes only subscriptions registered under the exact pattern string, not pattern-matching ones):

```go
removed := bus.Forget("orders.*")
```

## Bus API

| Method | Notes |
|---|---|
| `bus.Listen(pattern, l) Subscription` | Register a listener; panics on empty pattern or nil listener. On a closed bus returns an inert handle |
| `bus.Forget(pattern) int` | Remove every subscription whose pattern equals `pattern`; returns the count removed |
| `bus.Dispatch(ctx, e) error` | Fan the event out to matching listeners (sync) or queue it (async). `ErrClosed` after close |
| `bus.Patterns() []string` | Patterns of all current subscriptions |
| `bus.Close(ctx) error` | Idempotent; clears subscriptions. The async wrapper drains the queue first |

`Subscription` exposes `Cancel()` (idempotent) and `Pattern() string`.

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

## Context propagation

`events.ContextWithBus(ctx, bus)` attaches a `Bus` to a context for handlers that prefer pulling it from `context.Context`; `events.FromContext(ctx)` retrieves it (`ok == false` when absent). `events.FromApp(app)` is the canonical way to retrieve the bus published by `events.Module` (under `events.StoreKey`).

## Lifecycle

`events.Module` publishes the bus under `events.StoreKey` and registers `Bus.Close` as an `OnShutdown` callback. For the async bus, `Close` drains in-flight jobs (subject to the shutdown context). Only one `events.Module` may be initialised per App — a second init returns `events: Module already initialised`.

## Out of scope

- **Persistent queue** — handled by the upcoming `queue` module (`v0.11.x`). The async wrapper is in-memory only.
- **Schema validation** — handled by the upcoming `validation` module (`v0.9.0`).
- **Dead-letter** — the async wrapper passes errors to `OnError`; durable retry is a queue concern.
