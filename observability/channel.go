package observability

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/godx-jp/godx-platform-framework/framework"
)

// PrimaryChannel is the reserved name of the default channel — the one
// returned by [Provider.Logger]. Extra channels registered via [NewChannel]
// must use a different name.
const PrimaryChannel = "primary"

// NewChannel returns a framework module that registers an additional named
// channel on top of the primary one. Channels are a logging concept — each
// channel has its own slog handler, but traces and metrics continue to flow
// through the primary provider so distributed traces are not duplicated.
//
// Usage (Laravel-style — `Log::channel('audit')->info(...)`):
//
//	app := framework.New("svc", "1.0.0").
//	    Use(observability.Module).                                    // primary channel from env
//	    Use(observability.NewChannel("audit", observability.Config{   // extra channel
//	        Driver:      observability.DriverFile,
//	        LogFilePath: "/var/log/svc/audit.log",
//	    })).
//	    Use(observability.NewChannel("billing", observability.Config{
//	        Driver:       observability.DriverOTLP,
//	        OTLPEndpoint: "billing-collector:4317",
//	    }))
//
//	// later, in a handler:
//	obs := observability.FromContext(ctx)
//	obs.Logger().InfoContext(ctx, "normal log")
//	obs.Channel("audit").InfoContext(ctx, "user X did Y")
//	obs.Channel("billing").InfoContext(ctx, "payment", "amount", 100)
//
// Order matters: [Module] must be registered before [NewChannel] modules so
// the primary provider exists at the time the extra channel is wired in.
func NewChannel(name string, cfg Config) framework.Module {
	return &channelModule{name: name, cfg: cfg}
}

type channelModule struct {
	name string
	cfg  Config
}

func (m *channelModule) Name() string { return "observability.channel:" + m.name }

func (m *channelModule) Init(ctx context.Context, app *framework.App) error {
	if m.name == "" {
		return fmt.Errorf("observability: channel name must not be empty")
	}
	if m.name == PrimaryChannel {
		return fmt.Errorf("observability: channel name %q is reserved for the primary channel", PrimaryChannel)
	}

	cfg := m.cfg
	if cfg.ServiceName == "" {
		cfg.ServiceName = app.Name()
	}
	if cfg.ServiceVersion == "" {
		cfg.ServiceVersion = app.Version()
	}
	if cfg.Environment == "" {
		cfg.Environment = "dev"
	}

	channelProvider, err := NewProvider(ctx, cfg)
	if err != nil {
		return fmt.Errorf("observability: channel %q: %w", m.name, err)
	}

	primary, ok := lookupProvider(app)
	if !ok {
		return fmt.Errorf("observability: channel %q registered before Module — call app.Use(observability.Module) first", m.name)
	}
	primary.registerChannel(m.name, channelProvider.logger)

	app.OnShutdown(channelProvider.Shutdown)
	return nil
}

// --- registry attached to Provider.

type channelRegistry struct {
	mu  sync.RWMutex
	mp  map[string]*slog.Logger
}

func (r *channelRegistry) set(name string, logger *slog.Logger) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.mp == nil {
		r.mp = make(map[string]*slog.Logger)
	}
	r.mp[name] = logger
}

func (r *channelRegistry) get(name string) (*slog.Logger, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	l, ok := r.mp[name]
	return l, ok
}

func (r *channelRegistry) names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.mp)+1)
	out = append(out, PrimaryChannel)
	for k := range r.mp {
		out = append(out, k)
	}
	return out
}

// lookupProvider returns the active observability provider on the app, if
// the [Module] has been initialised on it. Returns false otherwise.
func lookupProvider(app *framework.App) (*Provider, bool) {
	v, ok := app.Lookup(StoreKey)
	if !ok {
		return nil, false
	}
	p, ok := v.(*Provider)
	return p, ok
}
