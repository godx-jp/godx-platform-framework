package database

import (
	"context"
	"fmt"
	"log/slog"

	ddriver "github.com/godx-jp/godx-platform-framework/database/driver"
	"github.com/godx-jp/godx-platform-framework/framework"
	godxhealth "github.com/godx-jp/godx-platform-framework/health"
	godxobs "github.com/godx-jp/godx-platform-framework/observability"
	"go.opentelemetry.io/otel"
)

// Module loads configuration from the environment and publishes a *Manager.
var Module framework.Module = envModule{}

type envModule struct{}

func (envModule) Name() string { return "database" }

func (envModule) Init(ctx context.Context, app *framework.App) error {
	return initFromConfig(ctx, app, LoadConfigFromEnv())
}

func initFromConfig(ctx context.Context, app *framework.App, cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if _, exists := app.Lookup(StoreKey); exists {
		return fmt.Errorf("database: Module already initialised")
	}

	obs := ddriver.ObservabilitySpec{
		LogQueries:     cfg.LogQueries,
		SlowThreshold:  cfg.LogSlow,
		LogArgs:        cfg.LogArgs,
		TraceQueries:   cfg.TraceQueries,
		MetricsEnabled: cfg.MetricsEnabled,
		Logger:         slog.Default(),
		TracerProvider: otel.GetTracerProvider(),
		MeterProvider:  otel.GetMeterProvider(),
	}
	if prov := godxobs.FromApp(app); prov != nil {
		obs.Logger = prov.Logger()
	}

	mgr := NewManager()
	mgr.configureRouting(cfg)

	for name, cc := range cfg.Connections {
		conn, err := buildConnection(ctx, name, cc, obs)
		if err != nil {
			_ = mgr.Shutdown(ctx)
			return fmt.Errorf("database: build connection %q: %w", name, err)
		}
		if err := mgr.AddConnection(conn); err != nil {
			_ = mgr.Shutdown(ctx)
			return err
		}
	}
	if err := mgr.SetDefault(cfg.DefaultConnection); err != nil {
		_ = mgr.Shutdown(ctx)
		return err
	}

	registerHealthProbes(app, mgr, cfg)
	startMetrics(mgr, cfg, app)

	app.Store(StoreKey, mgr)
	app.OnShutdown(mgr.Shutdown)
	return nil
}

func buildConnection(ctx context.Context, name string, cc ConnectionConfig, obs ddriver.ObservabilitySpec) (*Connection, error) {
	spec := ddriver.Spec{
		ConnectionName:    name,
		Name:              cc.Driver,
		URL:               cc.URL,
		MaxConns:          cc.MaxConns,
		MinConns:          cc.MinConns,
		MaxConnLifetime:   cc.MaxConnLifetime,
		MaxConnIdleTime:   cc.MaxConnIdleTime,
		HealthCheckPeriod: cc.HealthCheckPeriod,
		Obs:               obs,
	}
	h, err := ddriver.New(ctx, spec)
	if err != nil {
		return nil, err
	}
	return newConnection(name, h), nil
}

// BuildConnection constructs a single Connection without the module.
func BuildConnection(ctx context.Context, name string, cc ConnectionConfig, obs ddriver.ObservabilitySpec) (*Connection, error) {
	return buildConnection(ctx, name, cc, obs)
}

// ModuleWithConfig uses explicit Config instead of env.
func ModuleWithConfig(cfg Config) framework.Module {
	return &fixedModule{cfg: cfg}
}

type fixedModule struct{ cfg Config }

func (f *fixedModule) Name() string { return "database" }

func (f *fixedModule) Init(ctx context.Context, app *framework.App) error {
	return initFromConfig(ctx, app, f.cfg)
}

func registerHealthProbes(app *framework.App, mgr *Manager, cfg Config) {
	reg, err := godxhealth.FromApp(app)
	if err != nil {
		return
	}
	if write, err := mgr.Write(); err == nil {
		w := write
		reg.RegisterProbe("database:write:"+w.Name(), func(ctx context.Context) error {
			return w.Ping(ctx)
		})
	}
	for _, rn := range cfg.ReadConnections {
		readName := rn
		reg.RegisterProbe("database:read:"+readName, func(ctx context.Context) error {
			c, err := mgr.Connection(readName)
			if err != nil {
				return err
			}
			return c.Ping(ctx)
		})
	}
}

func startMetrics(mgr *Manager, cfg Config, app *framework.App) {
	if !cfg.MetricsEnabled {
		return
	}
	prov := godxobs.FromApp(app)
	if prov == nil {
		return
	}
	col, err := newMetricsCollector(prov.Meter())
	if err != nil {
		return
	}
	collectorCtx, cancel := context.WithCancel(context.Background())
	mgr.setMetricsStop(cancel)
	go col.run(collectorCtx, mgr, cfg.MetricsInterval)
}
