package scheduler

import (
	"context"
	"fmt"

	"github.com/godx-jp/godx-platform-framework/framework"
)

type contextKey struct{}

// ContextWithScheduler returns a derived context carrying sched.
func ContextWithScheduler(ctx context.Context, sched *Scheduler) context.Context {
	return context.WithValue(ctx, contextKey{}, sched)
}

// FromContext retrieves the Scheduler attached to ctx.
func FromContext(ctx context.Context) (*Scheduler, bool) {
	if ctx == nil {
		return nil, false
	}
	sched, ok := ctx.Value(contextKey{}).(*Scheduler)
	return sched, ok
}

// FromApp returns the Scheduler installed by scheduler.Module.
func FromApp(app *framework.App) (*Scheduler, error) {
	v, ok := app.Lookup(StoreKey)
	if !ok {
		return nil, fmt.Errorf("scheduler: Module has not been initialised on this App (did you call app.Use(scheduler.Module).Init(ctx)?)")
	}
	sched, ok := v.(*Scheduler)
	if !ok {
		return nil, fmt.Errorf("scheduler: %s framework Store entry is not a *Scheduler (%T)", StoreKey, v)
	}
	return sched, nil
}
