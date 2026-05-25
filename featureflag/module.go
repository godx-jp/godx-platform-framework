package featureflag

import (
	"context"
	"fmt"

	"github.com/godx-jp/godx-platform-framework/config"
	fdriver "github.com/godx-jp/godx-platform-framework/featureflag/driver"
	"github.com/godx-jp/godx-platform-framework/framework"
)

// StoreKey is the framework Store key for the Evaluator.
const StoreKey = "godx.featureflag.evaluator"

// Module is the default featureflag module (config driver).
var Module framework.Module = envModule{}

type envModule struct{}

func (envModule) Name() string { return "featureflag" }

func (envModule) Init(ctx context.Context, app *framework.App) error {
	return initFromConfig(ctx, app, LoadConfigFromEnv(), nil)
}

// ModuleWithConfig returns a Module using cfg. repo is required when
// Driver is config; pass config.Repository from config.FromApp.
func ModuleWithConfig(cfg Config, repo *config.Repository) framework.Module {
	return &fixedModule{cfg: cfg, repo: repo}
}

type fixedModule struct {
	cfg  Config
	repo *config.Repository
}

func (f *fixedModule) Name() string { return "featureflag" }

func (f *fixedModule) Init(ctx context.Context, app *framework.App) error {
	return initFromConfig(ctx, app, f.cfg, f.repo)
}

func initFromConfig(ctx context.Context, app *framework.App, cfg Config, repo *config.Repository) error {
	cfg = cfg.withDefaults()
	if err := cfg.Validate(); err != nil {
		return err
	}
	if _, exists := app.Lookup(StoreKey); exists {
		return fmt.Errorf("featureflag: Module already initialised (only one featureflag.Module per App)")
	}
	if cfg.Driver == fdriver.DriverConfig && repo == nil {
		repoVal, err := config.FromApp(app)
		if err != nil {
			return fmt.Errorf("featureflag: config driver requires config.Module: %w", err)
		}
		repo = repoVal
	}
	spec := fdriver.Spec{
		Name:     cfg.Driver,
		Prefix:   cfg.Prefix,
		Repo:     repo,
		SDKKey:   cfg.SDKKey,
		Endpoint: cfg.Endpoint,
		Project:  cfg.Project,
		AppName:  cfg.AppName,
	}
	provider, err := fdriver.New(ctx, spec)
	if err != nil {
		return fmt.Errorf("featureflag: build provider: %w", err)
	}
	eval, err := NewEvaluator(EvaluatorOptions{
		Provider:     provider,
		CacheEnabled: cfg.Cache,
		CacheTTL:     cfg.CacheTTL,
	})
	if err != nil {
		_ = provider.Shutdown(ctx)
		return err
	}
	app.Store(StoreKey, eval)
	app.OnShutdown(eval.Shutdown)
	return nil
}
