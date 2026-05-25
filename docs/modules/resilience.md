# Resilience

> Shared retry, circuit-breaker, timeout, and bulkhead primitives.

## Packages

| File | Type | Purpose |
|------|------|---------|
| `retry.go` | `RetryConfig`, `Do` | Exponential backoff with optional jitter |
| `circuitbreaker.go` | `CircuitBreaker` | Failure-count breaker with reset timeout |
| `timeout.go` | `WithTimeout` | Context deadline wrapper |
| `bulkhead.go` | `Bulkhead` | Concurrency limiter |

## Quick start

```go
import "github.com/godx-jp/godx-platform-framework/resilience"

err := resilience.Do(ctx, resilience.RetryConfig{
    MaxAttempts: 4,
    Backoff:     100 * time.Millisecond,
    Jitter:      true,
}, func(ctx context.Context) error {
    return call(ctx)
})

cb := resilience.NewCircuitBreaker(resilience.CircuitBreakerConfig{
    MaxFailures:  5,
    ResetTimeout: 30 * time.Second,
})
if err := cb.Allow(); err != nil { return err }
defer func() {
    if err != nil { cb.Failure() } else { cb.Success() }
}()

bh := resilience.NewBulkhead(8)
return bh.Run(ctx, fn)
```

## httpclient integration

The `httpclient/drivers/resilient` driver uses this package internally. The public httpclient API is unchanged — swap continues to be `HTTPCLIENT_DEFAULT=resilient`.

## Errors

| Error | When |
|-------|------|
| `resilience.ErrOpen` | Circuit breaker is open |
| `resilience.ErrBulkheadFull` | No bulkhead slot available (non-blocking acquire) |
