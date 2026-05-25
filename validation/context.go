package validation

import (
	"context"
	"fmt"

	"github.com/godx-jp/godx-platform-framework/framework"
)

type contextKey struct{}

// ContextWithValidator returns a derived context carrying v.
func ContextWithValidator(ctx context.Context, v *Validator) context.Context {
	return context.WithValue(ctx, contextKey{}, v)
}

// FromContext retrieves the Validator attached to ctx. ok == false
// when none is present.
func FromContext(ctx context.Context) (*Validator, bool) {
	if ctx == nil {
		return nil, false
	}
	v, ok := ctx.Value(contextKey{}).(*Validator)
	return v, ok
}

// FromApp returns the Validator published by validation.Module on app.
func FromApp(app *framework.App) (*Validator, error) {
	v, ok := app.Lookup(StoreKey)
	if !ok {
		return nil, fmt.Errorf("validation: Module has not been initialised on this App")
	}
	val, ok := v.(*Validator)
	if !ok {
		return nil, fmt.Errorf("validation: %s framework Store entry is not a *Validator (%T)", StoreKey, v)
	}
	return val, nil
}
