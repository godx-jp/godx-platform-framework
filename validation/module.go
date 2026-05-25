package validation

import (
	"context"
	"fmt"

	"github.com/godx-jp/godx-platform-framework/framework"
)

// StoreKey is the framework Store key under which the default
// Validator is published.
const StoreKey = "godx.validation.validator"

// Module is the default validation module — wires a Validator
// initialised with built-in rules and the English translator into
// the framework App.
var Module framework.Module = envModule{}

type envModule struct{}

func (envModule) Name() string { return "validation" }

func (envModule) Init(ctx context.Context, app *framework.App) error {
	return initModule(ctx, app, New())
}

// ModuleWithValidator returns a framework.Module that publishes the
// supplied Validator. Use this for tests or to register custom
// rules / translators before wiring.
func ModuleWithValidator(v *Validator) framework.Module {
	return &fixedModule{v: v}
}

type fixedModule struct{ v *Validator }

func (f *fixedModule) Name() string { return "validation" }

func (f *fixedModule) Init(ctx context.Context, app *framework.App) error {
	return initModule(ctx, app, f.v)
}

func initModule(_ context.Context, app *framework.App, v *Validator) error {
	if v == nil {
		return fmt.Errorf("validation: nil Validator")
	}
	if _, exists := app.Lookup(StoreKey); exists {
		return fmt.Errorf("validation: Module already initialised (only one validation.Module per App)")
	}
	app.Store(StoreKey, v)
	return nil
}
