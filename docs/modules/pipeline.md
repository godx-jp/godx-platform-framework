# Pipeline

> Laravel's `Pipeline` facade reimagined for Go. Chain typed
> transformer stages around a single value; each stage can
> short-circuit or pass control to the next.

## Concepts

A `Pipeline[T]` is a builder. Append `Stage[T]` functions with `Through`, then compile into a `Pipe[T]` with `Then(final)` or `ThenReturn()`. Each stage receives the current value and a `Next[T]` continuation; calling `next(ctx, value)` invokes the rest of the chain (and ultimately the final closure). Returning an error short-circuits — sibling stages and the final closure do not run.

```
Pipeline.Through(s1, s2, s3).Then(final)
                │
                ▼  on invocation:
   s1.pre → s2.pre → s3.pre → final → s3.post → s2.post → s1.post
```

## Quick start

```go
type Order struct { Total int }

withCoupon := func(ctx context.Context, o *Order, next pipeline.Next[*Order]) (*Order, error) {
    o.Total -= 100
    return next(ctx, o)
}
withVAT := func(ctx context.Context, o *Order, next pipeline.Next[*Order]) (*Order, error) {
    o.Total = o.Total * 110 / 100
    return next(ctx, o)
}

o, err := pipeline.New[*Order]().
    Through(withCoupon, withVAT).
    Then(func(ctx context.Context, o *Order) (*Order, error) { return o, nil })(
    ctx, &Order{Total: 1000},
)
```

## API

`Pipeline[T]` is generic over the value type `T` that flows through the chain. Every stage and the final closure are `context.Context`-first.

| Symbol | Signature | Notes |
|---|---|---|
| `New[T]()` | `*Pipeline[T]` | Empty builder. The zero `Pipeline[T]` is also usable |
| `(*Pipeline[T]).Through(stages ...Stage[T])` | `*Pipeline[T]` | Appends stages in order; nil stages are skipped; chainable |
| `(*Pipeline[T]).Then(final func(ctx, T) (T, error))` | `Pipe[T]` | Compiles the chain with a terminal closure; nil `final` becomes passthrough |
| `(*Pipeline[T]).ThenReturn()` | `Pipe[T]` | Compiles with an identity terminal — value returned unchanged |
| `(*Pipeline[T]).Stages()` | `int` | Number of registered stages — for diagnostics |
| `Stage[T]` | `func(ctx, value T, next Next[T]) (T, error)` | One transformer; call `next` to continue or return early to short-circuit |
| `Next[T]` | `func(ctx, value T) (T, error)` | Continuation invoking the rest of the chain |
| `Pipe[T]` | `func(ctx, value T) (T, error)` | Compiled executor returned by `Then` / `ThenReturn` |
| `FuncStage[T](fn func(ctx, T))` | `Stage[T]` | Side-effect-only stage that always delegates |
| `Chain(final http.Handler, stages ...HTTPStage)` | `http.Handler` | Composes net/http middlewares right-to-left |
| `HTTPStage` | `func(next http.Handler) http.Handler` | Standard net/http middleware shape |

## Short-circuiting

```go
auth := func(ctx context.Context, req *http.Request, next pipeline.Next[*http.Request]) (*http.Request, error) {
    if req.Header.Get("Authorization") == "" {
        return nil, errors.New("unauthenticated")
    }
    return next(ctx, req)
}
```

A stage that returns a non-nil error stops the chain — subsequent stages and the final closure are skipped. Use this for guards (auth, feature flag, rate limit).

## Side-effect-only stages

`FuncStage(fn)` wraps a closure that always delegates — handy for logging, metrics, tracing where you don't want to write `return next(ctx, v)` every time.

```go
log := pipeline.FuncStage(func(ctx context.Context, o *Order) {
    slog.InfoContext(ctx, "order", "total", o.Total)
})
```

## net/http middleware compatibility

`pipeline.Chain(final, stages...)` composes net/http middlewares right-to-left so the first argument runs outermost (matches chi/echo/gin `Use(...)` conventions):

```go
h := pipeline.Chain(
    handler,
    auth,
    requestLogger,
    rateLimit,
)
```

`HTTPStage` is the standard `func(next http.Handler) http.Handler` shape — no adapter needed for any existing middleware in the ecosystem.

## ThenReturn vs Then

| Method | Final closure |
|---|---|
| `Then(fn)` | runs `fn(ctx, value)` after every stage; useful for "perform the action" terminators |
| `ThenReturn()` | identity — returns the value unchanged once the chain finishes; useful for side-effect-only chains |

## Laravel API mapping

| Laravel | Framework |
|---|---|
| `Pipeline::send($x)->through([m1, m2])->thenReturn()` | `pipeline.New[T]().Through(s1, s2).ThenReturn()(ctx, x)` |
| `Pipeline::send($x)->through([m1, m2])->then(fn $y)` | `pipeline.New[T]().Through(s1, s2).Then(fn)(ctx, x)` |
| Closure `fn($x, $next) { return $next($x); }` | `pipeline.Stage[T]` — `func(ctx, T, next) (T, error)` |
| `App\Http\Kernel` middleware groups | compose multiple slices of `Stage[T]` and `Through(...)` them in order |

## Out of scope

- **Concurrent stages** — Pipeline is serial by design. For fan-out, use `events.NewAsync` or `errgroup`.
- **Retries / circuit-breakers** — the upcoming `resilience` module (v0.10.4).
- **DI** — Pipeline takes already-built stage closures; service location stays elsewhere.
