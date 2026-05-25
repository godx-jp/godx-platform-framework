package resilience

import (
	"context"
	"errors"
)

// ErrBulkheadFull is returned when no bulkhead slot is available.
var ErrBulkheadFull = errors.New("resilience: bulkhead full")

// Bulkhead limits concurrent executions.
type Bulkhead struct {
	sem chan struct{}
}

// NewBulkhead returns a bulkhead allowing maxConcurrent parallel calls.
func NewBulkhead(maxConcurrent int) *Bulkhead {
	if maxConcurrent <= 0 {
		maxConcurrent = 1
	}
	return &Bulkhead{sem: make(chan struct{}, maxConcurrent)}
}

// Acquire reserves a slot or returns ErrBulkheadFull when ctx expires.
func (b *Bulkhead) Acquire(ctx context.Context) (release func(), err error) {
	select {
	case b.sem <- struct{}{}:
		return func() { <-b.sem }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		select {
		case b.sem <- struct{}{}:
			return func() { <-b.sem }, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			return nil, ErrBulkheadFull
		}
	}
}

// Run executes fn under bulkhead concurrency control.
func (b *Bulkhead) Run(ctx context.Context, fn func(context.Context) error) error {
	release, err := b.Acquire(ctx)
	if err != nil {
		return err
	}
	defer release()
	return fn(ctx)
}
