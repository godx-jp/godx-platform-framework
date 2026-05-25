package observability

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"

	"github.com/godx-jp/godx-platform-framework/observability/drivers"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// Provider holds the live observability handles for a service. Obtain one via
// the framework module ([Module]) or directly with [NewProvider].
type Provider struct {
	cfg    Config
	driver drivers.Driver
	logger *slog.Logger
	tracer trace.Tracer
	meter  metric.Meter

	channels channelRegistry
}

// NewProvider constructs a provider for the given config. Most callers should
// register [Module] on a [framework.App] instead and let the framework call
// this for them.
func NewProvider(ctx context.Context, cfg Config) (*Provider, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	d, err := drivers.New(ctx, drivers.Spec{
		Name:               cfg.Driver,
		ServiceName:        cfg.ServiceName,
		ServiceVersion:     cfg.ServiceVersion,
		Environment:        cfg.Environment,
		LogLevel:           cfg.LogLevel,
		TraceSampleRate:    cfg.TraceSampleRate,
		OTLPEndpoint:       cfg.OTLPEndpoint,
		OTLPProtocol:       cfg.OTLPProtocol,
		OTLPInsecure:       cfg.OTLPInsecure,
		AWSRegion:          cfg.AWSRegion,
		CloudWatchLogGroup: cfg.CloudWatchLogGroup,
		LogFilePath:        cfg.LogFilePath,
		LogFileRotation:    cfg.LogFileRotation,
		LogFileMaxSizeMB:   cfg.LogFileMaxSizeMB,
		LogFileMaxAgeDays:  cfg.LogFileMaxAgeDays,
		LogFileMaxBackups:  cfg.LogFileMaxBackups,
		LogFileCompress:    cfg.LogFileCompress,
		StackDrivers:       cfg.StackDrivers,
	})
	if err != nil {
		return nil, fmt.Errorf("observability: driver %q: %w", cfg.Driver, err)
	}

	// Wrap the slog handler so trace_id / correlation_id are injected
	// automatically from context.
	logger := slog.New(&contextHandler{inner: d.LoggerHandler()}).With(
		slog.String("service", cfg.ServiceName),
		slog.String("version", cfg.ServiceVersion),
		slog.String("env", cfg.Environment),
	)

	otel.SetTracerProvider(d.TracerProvider())
	otel.SetMeterProvider(d.MeterProvider())

	p := &Provider{
		cfg:    cfg,
		driver: d,
		logger: logger,
		tracer: d.TracerProvider().Tracer(cfg.ServiceName),
		meter:  d.MeterProvider().Meter(cfg.ServiceName),
	}
	setGlobalProvider(p)
	return p, nil
}

// Driver reports the active driver name (e.g. "otlp").
func (p *Provider) Driver() string { return p.cfg.Driver }

// Logger returns the contextual slog logger for the primary channel.
// Pre-decorated with service identity; trace_id / correlation_id are
// injected on each Handle call. Equivalent to Channel(PrimaryChannel).
func (p *Provider) Logger() *slog.Logger { return p.logger }

// Channel returns the slog logger for the given channel name (Laravel-style
// `Log::channel('audit')->info(...)`). Pass "" or [PrimaryChannel] to get
// the same logger as [Provider.Logger].
//
// If the channel has not been registered with [NewChannel], the primary
// logger is returned and a warning is emitted — channels are best-effort by
// design so calling code does not need to nil-check.
func (p *Provider) Channel(name string) *slog.Logger {
	if name == "" || name == PrimaryChannel {
		return p.logger
	}
	if logger, ok := p.channels.get(name); ok {
		return logger
	}
	p.logger.Warn("observability: unknown channel, falling back to primary",
		"channel", name,
		"available", p.channels.names(),
	)
	return p.logger
}

// Channels reports every registered channel name, including the primary.
// Useful for diagnostics or admin endpoints.
func (p *Provider) Channels() []string { return p.channels.names() }

// registerChannel is the internal hook used by [NewChannel] modules.
func (p *Provider) registerChannel(name string, logger *slog.Logger) {
	p.channels.set(name, logger)
}

// Tracer returns the OTel tracer scoped to the service name.
func (p *Provider) Tracer() trace.Tracer { return p.tracer }

// Meter returns the OTel meter scoped to the service name.
func (p *Provider) Meter() metric.Meter { return p.meter }

// Shutdown flushes pending telemetry and tears the driver down.
func (p *Provider) Shutdown(ctx context.Context) error {
	clearGlobalProvider(p)
	return p.driver.Shutdown(ctx)
}

// --- contextHandler injects trace + correlation IDs into every log record.

type contextHandler struct{ inner slog.Handler }

func (c *contextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return c.inner.Enabled(ctx, level)
}

func (c *contextHandler) Handle(ctx context.Context, r slog.Record) error {
	if span := trace.SpanFromContext(ctx); span != nil && span.SpanContext().IsValid() {
		sc := span.SpanContext()
		r.AddAttrs(
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
	}
	if cid := CorrelationIDFromContext(ctx); cid != "" {
		r.AddAttrs(slog.String("correlation_id", cid))
	}
	return c.inner.Handle(ctx, r)
}

func (c *contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &contextHandler{inner: c.inner.WithAttrs(attrs)}
}

func (c *contextHandler) WithGroup(name string) slog.Handler {
	return &contextHandler{inner: c.inner.WithGroup(name)}
}

// --- noop fallback for FromContext when no provider was set.

var global atomic.Pointer[Provider]

func setGlobalProvider(p *Provider)   { global.Store(p) }
func clearGlobalProvider(p *Provider) { global.CompareAndSwap(p, nil) }

func globalProvider() *Provider {
	if p := global.Load(); p != nil {
		return p
	}
	// Construct a stdout fallback once, lazily, so library callers that
	// hold a context without an explicit provider still get readable logs.
	return fallback
}

var fallback = func() *Provider {
	cfg := Config{
		ServiceName:    "noop",
		ServiceVersion: "0.0.0",
		Environment:    "dev",
		Driver:         DriverStdout,
		LogLevel:       slog.LevelInfo,
	}
	p, _ := NewProvider(context.Background(), cfg)
	return p
}()
