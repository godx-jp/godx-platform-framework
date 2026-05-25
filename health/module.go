package health

import (
	"context"
	"fmt"

	"github.com/godx-jp/godx-platform-framework/framework"
)

const StoreKey = "godx.health.registry"

var Module framework.Module = module{}

type module struct{}

func (module) Name() string { return "health" }

func (module) Init(_ context.Context, app *framework.App) error {
	if _, exists := app.Lookup(StoreKey); exists {
		return fmt.Errorf("health: Module already initialised")
	}
	app.Store(StoreKey, NewRegistry())
	return nil
}

// ModuleWithRegistry publishes a pre-built Registry.
func ModuleWithRegistry(reg *Registry) framework.Module {
	return &fixedModule{reg: reg}
}

type fixedModule struct{ reg *Registry }

func (f *fixedModule) Name() string { return "health" }

func (f *fixedModule) Init(_ context.Context, app *framework.App) error {
	if f.reg == nil {
		return fmt.Errorf("health: nil Registry")
	}
	if _, exists := app.Lookup(StoreKey); exists {
		return fmt.Errorf("health: Module already initialised")
	}
	app.Store(StoreKey, f.reg)
	return nil
}
