package observability

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/godx-jp/godx-platform-framework/framework"
)

// PrimaryChannel is the reserved name of the default channel — the one
// returned by [Provider.Logger]. Extra channels registered via [NewChannel]
// must use a different name.
const PrimaryChannel = "primary"

// ChannelsEnvVar is the comma-separated list of extra channel names read by
// [ChannelsFromEnv]. Each name X expands to a per-channel env-var prefix
// `OBSERVABILITY_CHANNEL_<X>_` (X is upper-cased).
const ChannelsEnvVar = "OBSERVABILITY_CHANNELS"

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

// LoadChannelConfigFromEnv reads channel-scoped configuration from env vars
// prefixed with `OBSERVABILITY_CHANNEL_<NAME>_`. Mirrors
// [LoadConfigFromEnv] but with per-channel keys and Laravel-style
// per-channel `LOG_LEVEL`. Unlike the primary loader, the OTLP vars are
// also namespaced under the channel (because OTEL_EXPORTER_OTLP_* is a
// global convention that cannot be repeated per channel):
//
//	OBSERVABILITY_CHANNEL_AUDIT_DRIVER=file
//	OBSERVABILITY_CHANNEL_AUDIT_LOG_LEVEL=warn
//	OBSERVABILITY_CHANNEL_AUDIT_LOG_FILE_PATH=/var/log/audit.log
//	OBSERVABILITY_CHANNEL_AUDIT_LOG_FILE_ROTATION=daily
//
//	OBSERVABILITY_CHANNEL_BILLING_DRIVER=otlp
//	OBSERVABILITY_CHANNEL_BILLING_OTLP_ENDPOINT=billing-collector:4317
//	OBSERVABILITY_CHANNEL_BILLING_OTLP_PROTOCOL=grpc
//
// Defaults for missing values mirror the primary loader (driver=stdout,
// level=info, file rotation=daily, OTLP protocol=grpc, etc.).
func LoadChannelConfigFromEnv(name string) Config {
	pfx := "OBSERVABILITY_CHANNEL_" + envSegment(name) + "_"
	return Config{
		Driver:             getEnv(pfx+"DRIVER", DriverStdout),
		LogLevel:           parseLogLevel(getEnv(pfx+"LOG_LEVEL", "info")),
		TraceSampleRate:    parseFloat(getEnv(pfx+"TRACE_SAMPLE_RATE", "1.0"), 1.0),
		OTLPEndpoint:       os.Getenv(pfx + "OTLP_ENDPOINT"),
		OTLPProtocol:       getEnv(pfx+"OTLP_PROTOCOL", "grpc"),
		OTLPInsecure:       parseBool(getEnv(pfx+"OTLP_INSECURE", "true"), true),
		AWSRegion:          os.Getenv(pfx + "AWS_REGION"),
		CloudWatchLogGroup: os.Getenv(pfx + "CLOUDWATCH_LOG_GROUP"),
		LogFilePath:        os.Getenv(pfx + "LOG_FILE_PATH"),
		LogFileRotation:    getEnv(pfx+"LOG_FILE_ROTATION", "daily"),
		LogFileMaxSizeMB:   parseInt(getEnv(pfx+"LOG_FILE_MAX_SIZE_MB", "100"), 100),
		LogFileMaxAgeDays:  parseInt(getEnv(pfx+"LOG_FILE_MAX_AGE_DAYS", "14"), 14),
		LogFileMaxBackups:  parseInt(getEnv(pfx+"LOG_FILE_MAX_BACKUPS", "0"), 0),
		LogFileCompress:    parseBool(getEnv(pfx+"LOG_FILE_COMPRESS", "true"), true),
		StackDrivers:       parseCSV(os.Getenv(pfx + "STACK_DRIVERS")),
	}
}

// envSegment normalises a channel name to its env-var segment: uppercase,
// hyphens/spaces converted to underscores. `audit-trail` → `AUDIT_TRAIL`.
func envSegment(name string) string {
	return strings.ToUpper(strings.NewReplacer("-", "_", " ", "_").Replace(name))
}

// ChannelsFromEnv returns a framework module that registers every channel
// declared via [ChannelsEnvVar] (`OBSERVABILITY_CHANNELS`). Per-channel
// configuration uses keys prefixed with `OBSERVABILITY_CHANNEL_<NAME>_` —
// see [LoadChannelConfigFromEnv].
//
// Usage (no Go code per channel — pure 12-factor config):
//
//	app := framework.New("svc", "1.0.0").
//	    Use(observability.Module).
//	    Use(observability.ChannelsFromEnv())
//
// With:
//
//	OBSERVABILITY_DRIVER=stdout
//	OBSERVABILITY_CHANNELS=audit,billing
//	OBSERVABILITY_CHANNEL_AUDIT_DRIVER=file
//	OBSERVABILITY_CHANNEL_AUDIT_LOG_LEVEL=warn
//	OBSERVABILITY_CHANNEL_AUDIT_LOG_FILE_PATH=/var/log/audit.log
//	OBSERVABILITY_CHANNEL_BILLING_DRIVER=otlp
//	OBSERVABILITY_CHANNEL_BILLING_OTLP_ENDPOINT=billing-collector:4317
//
// The returned module is a no-op when `OBSERVABILITY_CHANNELS` is unset or
// empty, so it is safe to leave in `main` for services that have not
// declared any extra channels yet. Heavy drivers still require their own
// blank import in consumer code.
//
// Order matters: like [NewChannel], this module must be registered AFTER
// [Module] so the primary provider exists at wire-up time.
func ChannelsFromEnv() framework.Module { return &envChannelsMod{} }

type envChannelsMod struct{}

func (envChannelsMod) Name() string { return "observability.channels-from-env" }

func (envChannelsMod) Init(ctx context.Context, app *framework.App) error {
	names := parseCSV(os.Getenv(ChannelsEnvVar))
	if len(names) == 0 {
		return nil
	}

	primary, ok := lookupProvider(app)
	if !ok {
		return fmt.Errorf("observability: ChannelsFromEnv must be registered after observability.Module")
	}

	seen := make(map[string]struct{}, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if name == PrimaryChannel {
			return fmt.Errorf("observability: channel name %q (from %s) is reserved for the primary channel", PrimaryChannel, ChannelsEnvVar)
		}
		if _, dup := seen[name]; dup {
			return fmt.Errorf("observability: channel %q listed twice in %s", name, ChannelsEnvVar)
		}
		seen[name] = struct{}{}

		cfg := LoadChannelConfigFromEnv(name)
		if cfg.ServiceName == "" {
			cfg.ServiceName = app.Name()
		}
		if cfg.ServiceVersion == "" {
			cfg.ServiceVersion = app.Version()
		}
		if cfg.Environment == "" {
			cfg.Environment = getEnv("DEPLOYMENT_ENVIRONMENT", "dev")
		}

		channelProvider, err := NewProvider(ctx, cfg)
		if err != nil {
			return fmt.Errorf("observability: channel %q (from %s): %w", name, ChannelsEnvVar, err)
		}
		primary.registerChannel(name, channelProvider.logger)
		app.OnShutdown(channelProvider.Shutdown)
	}
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
