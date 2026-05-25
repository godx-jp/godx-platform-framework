package pipeline

import "context"

// Next advances to the next stage in the chain. Stages call it once
// to delegate further processing, or skip it to short-circuit.
type Next[T any] func(ctx context.Context, value T) (T, error)

// Stage is a single transformer. It receives the current value plus
// a closure that runs the rest of the chain. Returning an error
// short-circuits — subsequent stages and the final closure do not
// run. Stages must respect ctx.Err(); long-running work should bail
// out when the context is canceled.
type Stage[T any] func(ctx context.Context, value T, next Next[T]) (T, error)

// Pipeline is the chained-stage executor. Zero value is usable —
// New[T]() is a thin alias for clarity.
type Pipeline[T any] struct {
	stages []Stage[T]
}

// New constructs an empty Pipeline.
func New[T any]() *Pipeline[T] { return &Pipeline[T]{} }

// Through appends stages to the chain. Stages run in append order.
// Through is chainable; nil stages are skipped.
func (p *Pipeline[T]) Through(stages ...Stage[T]) *Pipeline[T] {
	for _, s := range stages {
		if s == nil {
			continue
		}
		p.stages = append(p.stages, s)
	}
	return p
}

// Pipe is the compiled closure shape — pass an input, get an output
// or an error. Returned by Then / ThenReturn.
type Pipe[T any] func(ctx context.Context, value T) (T, error)

// Then compiles the pipeline with a final closure that receives the
// fully-transformed value. The returned Pipe runs the chain end-to-end.
func (p *Pipeline[T]) Then(final func(ctx context.Context, value T) (T, error)) Pipe[T] {
	if final == nil {
		final = passthrough[T]
	}
	stages := make([]Stage[T], len(p.stages))
	copy(stages, p.stages)
	return func(ctx context.Context, value T) (T, error) {
		// Build the chain by closing over each stage right-to-left so
		// the leftmost stage runs first when Pipe is invoked.
		next := Next[T](final)
		for i := len(stages) - 1; i >= 0; i-- {
			stage := stages[i]
			cur := next
			next = func(ctx context.Context, v T) (T, error) {
				return stage(ctx, v, cur)
			}
		}
		return next(ctx, value)
	}
}

// ThenReturn compiles the pipeline with a passthrough final closure
// — the value is handed back unchanged after every stage has run.
// Use it when stages only produce side effects (logging, metrics).
func (p *Pipeline[T]) ThenReturn() Pipe[T] {
	return p.Then(passthrough[T])
}

// Stages returns the number of registered stages — handy for diagnostics.
func (p *Pipeline[T]) Stages() int { return len(p.stages) }

func passthrough[T any](_ context.Context, v T) (T, error) { return v, nil }
