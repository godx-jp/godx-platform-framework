package events

import (
	"context"
	"fmt"

	"github.com/godx-jp/godx-platform-framework/framework"
)

type contextKey struct{}

// ContextWithBus returns a derived context carrying bus.
func ContextWithBus(ctx context.Context, bus Bus) context.Context {
	return context.WithValue(ctx, contextKey{}, bus)
}

// FromContext retrieves the Bus attached to ctx by ContextWithBus.
// ok == false when no bus is present.
func FromContext(ctx context.Context) (Bus, bool) {
	if ctx == nil {
		return nil, false
	}
	bus, ok := ctx.Value(contextKey{}).(Bus)
	return bus, ok
}

// FromApp returns the Bus published by events.Module on app.
func FromApp(app *framework.App) (Bus, error) {
	v, ok := app.Lookup(StoreKey)
	if !ok {
		return nil, fmt.Errorf("events: Module has not been initialised on this App (did you call app.Use(events.Module).Init(ctx)?)")
	}
	bus, ok := v.(Bus)
	if !ok {
		return nil, fmt.Errorf("events: %s framework Store entry is not a Bus (%T)", StoreKey, v)
	}
	return bus, nil
}
