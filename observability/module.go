package observability

import (
	"context"

	"github.com/godx-jp/godx-platform-framework/framework"
)

// Module is the [framework.Module] that initialises observability from the
// process environment.
//
//	app := framework.New("svc", "1.0.0").Use(observability.Module)
//
// For explicit configuration in tests or non-framework apps, build a
// [*Provider] directly with [NewProvider].
var Module framework.Module = mod{}

type mod struct{}

func (mod) Name() string { return "observability" }

func (mod) Init(ctx context.Context, app *framework.App) error {
	cfg := LoadConfigFromEnv()
	cfg.ServiceName = app.Name()
	cfg.ServiceVersion = app.Version()

	p, err := NewProvider(ctx, cfg)
	if err != nil {
		return err
	}
	app.Store(StoreKey, p)
	app.OnShutdown(p.Shutdown)
	return nil
}
