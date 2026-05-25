# Resilience

> Standalone retry, circuit-breaker, timeout, and bulkhead primitives.
> No framework dependency, no drivers — import the package and call the
> functions directly. Used internally by `httpclient/drivers/resilient`.

## Concepts

`resilience` is a leaf utility package: every primitive is independent
and holds its own state. There is no `Manager`, no `framework.Module`,
and nothing to register — you construct breakers/bulkheads yourself and
call the package-level `Do` / `WithTimeout` helpers inline.

```
resilience
 ├─ Do(ctx, RetryConfig, fn)        — retry with fixed backoff + jitter
 ├─ WithTimeout(ctx, d, fn)         — context deadline wrapper
 ├─ CircuitBreaker (NewCircuitBreaker) — Allow / Success / Failure
 └─ Bulkhead (NewBulkhead)          — Acquire / Run concurrency limiter
```

## Quick start

Standalone usage — no `framework.App` involved:

```go
package main

import (
    "context"
    "time"

    "github.com/godx-jp/godx-platform-framework/resilience"
)

func call(ctx context.Context) error { return nil }

func main() {
    ctx := context.Background()

    // Retry up to 4 attempts, 100ms backoff between tries, with jitter.
    err := resilience.Do(ctx, resilience.RetryConfig{
        MaxAttempts: 4,
        Backoff:     100 * time.Millisecond,
        Jitter:      true,
    }, func(ctx context.Context) error {
        return call(ctx)
    })
    _ = err

    // Circuit breaker guarding a flaky dependency.
    cb := resilience.NewCircuitBreaker(resilience.CircuitBreakerConfig{
        MaxFailures:  5,
        ResetTimeout: 30 * time.Second,
    })
    if err := cb.Allow(); err == nil {
        if call(ctx) != nil {
            cb.Failure()
        } else {
            cb.Success()
        }
    }

    // Bulkhead capping concurrent calls at 8.
    bh := resilience.NewBulkhead(8)
    _ = bh.Run(ctx, call)

    // One-shot deadline.
    _ = resilience.WithTimeout(ctx, 2*time.Second, call)
}
```

## API

### Retry — `Do`

```go
func Do(ctx context.Context, cfg RetryConfig, fn func(context.Context) error) error

type RetryConfig struct {
    MaxAttempts int
    Backoff     time.Duration
    Jitter      bool
    Retryable   func(error) bool
}
```

`Do` runs `fn` up to `MaxAttempts` times. The wait between attempts is a
**fixed** `Backoff` duration (not exponential); when `Jitter` is true a
random `[0, Backoff)` is added to each wait. Defaults are applied when a
field is zero:

| Field | Zero-value behaviour |
|---|---|
| `MaxAttempts` | `<= 0` → `1` (single attempt, no retry) |
| `Backoff` | `<= 0` → `100ms` |
| `Retryable` | `nil` → retry every error **except** `context.Canceled` and `context.DeadlineExceeded` |

`Do` returns immediately on the first success, on a non-retryable error,
or with `ctx.Err()` if the context is canceled during a backoff wait.
The last observed error is returned when all attempts are exhausted.

### Circuit breaker

```go
func NewCircuitBreaker(cfg CircuitBreakerConfig) *CircuitBreaker

type CircuitBreakerConfig struct {
    MaxFailures   int                    // default 5
    ResetTimeout  time.Duration          // default 30s
    OnStateChange func(from, to State)   // optional; nil = silent (default)
}

func (b *CircuitBreaker) Allow() error   // ErrOpen while open, else nil
func (b *CircuitBreaker) Success()        // reset failure count, close
func (b *CircuitBreaker) Failure()        // record a failure; open at MaxFailures
func (b *CircuitBreaker) State() State    // current observable state
```

The breaker is a **consecutive-failure** counter. `Failure` increments
the count; once it reaches `MaxFailures` the breaker opens for
`ResetTimeout`. While open, `Allow` returns `ErrOpen`. After the reset
window elapses the next `Allow` permits a single probe call (the
**half-open** phase); a `Success` on that probe clears the count and
closes the breaker, while a `Failure` re-opens it for another
`ResetTimeout`.

#### Observable state — `State` and `OnStateChange`

```go
type State int // StateClosed, StateOpen, StateHalfOpen

func (s State) String() string // "closed" | "open" | "half-open"
```

The transitions the breaker makes are surfaced through the optional
`OnStateChange(from, to)` callback:

| Transition | Trigger |
|---|---|
| `StateClosed → StateOpen` | `MaxFailures` consecutive failures |
| `StateOpen → StateHalfOpen` | first `Allow`/`State` after `ResetTimeout` elapses |
| `StateHalfOpen → StateClosed` | a `Success` on the probe |
| `StateHalfOpen → StateOpen` | a `Failure` on the probe |

`OnStateChange` fires **exactly once per actual transition** — repeated
same-state operations (e.g. an `Allow` that finds the breaker still
open) do not call it. `nil` (the default) keeps the original behaviour
with zero overhead.

A breaker opening is a **leading indicator** of an unhealthy dependency:
failures already crossed the threshold and the breaker is now shedding
load. Wiring `OnStateChange` turns that silent event into an early
warning worth alerting on. The callback runs **outside** the breaker's
lock, so it is safe to log, emit a metric, or push to an error reporter
from inside it (it may even call back into the breaker):

```go
cb := resilience.NewCircuitBreaker(resilience.CircuitBreakerConfig{
    MaxFailures:  5,
    ResetTimeout: 30 * time.Second,
    OnStateChange: func(from, to resilience.State) {
        switch to {
        case resilience.StateOpen:
            // Leading indicator — alert / page / report on this.
            slog.Warn("payments breaker opened", "from", from, "to", to)
        case resilience.StateClosed:
            slog.Info("payments breaker recovered", "from", from, "to", to)
        }
    },
})
```

### Bulkhead

```go
func NewBulkhead(maxConcurrent int) *Bulkhead   // <= 0 → 1

func (b *Bulkhead) Acquire(ctx context.Context) (release func(), err error)
func (b *Bulkhead) Run(ctx context.Context, fn func(context.Context) error) error
```

`Bulkhead` is a counting semaphore capping concurrent executions.
`Acquire` is **non-blocking**: it returns `ErrBulkheadFull` when no slot
is free (it also honours `ctx` cancellation). Always call the returned
`release` when done. `Run` is the convenience wrapper — acquire, run
`fn`, release.

### Timeout

```go
func WithTimeout(ctx context.Context, d time.Duration, fn func(context.Context) error) error
```

Derives a `context.WithTimeout(ctx, d)` and runs `fn` under it. When
`d <= 0` it runs `fn` against the original `ctx` with no deadline.

## Error model

| Error | When |
|-------|------|
| `resilience.ErrOpen` | `CircuitBreaker.Allow` while the breaker is open |
| `resilience.ErrBulkheadFull` | `Bulkhead.Acquire`/`Run` with no free slot |

`Do` surfaces `fn`'s own error verbatim, or `ctx.Err()`
(`context.Canceled` / `context.DeadlineExceeded`) when the context ends
during a backoff wait. `WithTimeout` propagates whatever `fn` returns,
which is typically `context.DeadlineExceeded` when the derived deadline
fires.

## httpclient integration

The `httpclient/drivers/resilient` driver composes these primitives
internally. The public httpclient API is unchanged — selection stays
`HTTPCLIENT_DEFAULT=resilient`.

## Lifecycle

None. There is nothing to boot or shut down — primitives are plain
values you create where you need them. `CircuitBreaker` and `Bulkhead`
are safe for concurrent use; `Do`/`WithTimeout` are stateless.
