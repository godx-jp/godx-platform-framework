// Package pipeline implements a Laravel-style middleware chain.
// Stages observe and transform a value of type T as it flows
// through; each stage can short-circuit by returning an error or
// pass control to the next stage by invoking next.
//
//	type Order struct { Total int; Notes string }
//
//	withCoupon := pipeline.Stage[*Order](func(ctx context.Context, o *Order, next pipeline.Next[*Order]) (*Order, error) {
//	    o.Total -= 100
//	    return next(ctx, o)
//	})
//	withVAT := pipeline.Stage[*Order](func(ctx context.Context, o *Order, next pipeline.Next[*Order]) (*Order, error) {
//	    o.Total = o.Total * 110 / 100
//	    return next(ctx, o)
//	})
//
//	o, err := pipeline.New[*Order]().
//	    Through(withCoupon, withVAT).
//	    Then(func(ctx context.Context, o *Order) (*Order, error) {
//	        return o, nil
//	    })(ctx, &Order{Total: 1000})
//
// Laravel mapping:
//
//	Laravel                                       | Framework
//	----------------------------------------------|----------------------------
//	Pipeline::send($x)->through([...])->thenReturn() | pipeline.New[T]().Through(...).ThenReturn()(ctx, x)
//	Pipeline::send($x)->through([...])->then(fn $y) | pipeline.New[T]().Through(...).Then(fn)(ctx, x)
//	Closure-style middleware (fn($x, $next))      | pipeline.Stage[T] (ctx, T, next) → (T, error)
//
// The pipeline module is a tiny standalone utility — it does NOT
// require framework.App wiring. Reach for it whenever you have a
// linear sequence of transforms with optional short-circuiting.
package pipeline
