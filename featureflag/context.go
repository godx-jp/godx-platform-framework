package featureflag

import (
	"context"
	"fmt"

	"github.com/godx-jp/godx-platform-framework/framework"
)

type contextKey struct{}

// ContextWithEvaluator attaches eval to ctx.
func ContextWithEvaluator(ctx context.Context, eval *Evaluator) context.Context {
	return context.WithValue(ctx, contextKey{}, eval)
}

// FromContext retrieves the Evaluator from ctx.
func FromContext(ctx context.Context) (*Evaluator, bool) {
	if ctx == nil {
		return nil, false
	}
	eval, ok := ctx.Value(contextKey{}).(*Evaluator)
	return eval, ok
}

// FromApp returns the Evaluator installed by featureflag.Module.
func FromApp(app *framework.App) (*Evaluator, error) {
	v, ok := app.Lookup(StoreKey)
	if !ok {
		return nil, fmt.Errorf("featureflag: Module has not been initialised on this App (did you call app.Use(featureflag.Module).Init(ctx)?)")
	}
	eval, ok := v.(*Evaluator)
	if !ok {
		return nil, fmt.Errorf("featureflag: %s framework Store entry is not a *Evaluator (%T)", StoreKey, v)
	}
	return eval, nil
}
