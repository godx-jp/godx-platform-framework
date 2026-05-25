package observability

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"

	"github.com/godx-jp/godx-platform-framework/observability/backends"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// Provider holds the live observability handles for a service. Obtain one via
// the framework module ([Module]) or directly with [NewProvider].
type Provider struct {
	cfg     Config
	backend backends.Backend
	logger  *slog.Logger
	tracer  trace.Tracer
	meter   metric.Meter
}

// NewProvider constructs a provider for the given config. Most callers should
// register [Module] on a [framework.App] instead and let the framework call
// this for them.
func NewProvider(ctx context.Context, cfg Config) (*Provider, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	be, err := backends.New(ctx, backends.Spec{
		Name:            cfg.Backend,
		ServiceName:     cfg.ServiceName,
		ServiceVersion:  cfg.ServiceVersion,
		Environment:     cfg.Environment,
		LogLevel:        cfg.LogLevel,
		TraceSampleRate: cfg.TraceSampleRate,
		OTLPEndpoint:    cfg.OTLPEndpoint,
		OTLPProtocol:    cfg.OTLPProtocol,
		OTLPInsecure:    cfg.OTLPInsecure,
		AWSRegion:       cfg.AWSRegion,
		LogGroupName:    cfg.LogGroupName,
		FilePath:        cfg.FilePath,
		FileRotate:      cfg.FileRotate,
		FileMaxSizeMB:   cfg.FileMaxSizeMB,
		FileMaxAgeDays:  cfg.FileMaxAgeDays,
		FileMaxBackups:  cfg.FileMaxBackups,
		FileCompress:    cfg.FileCompress,
	})
	if err != nil {
		return nil, fmt.Errorf("observability: backend %q: %w", cfg.Backend, err)
	}

	// Wrap the slog handler so trace_id / correlation_id are injected
	// automatically from context.
	logger := slog.New(&contextHandler{inner: be.LoggerHandler()}).With(
		slog.String("service", cfg.ServiceName),
		slog.String("version", cfg.ServiceVersion),
		slog.String("env", cfg.Environment),
	)

	otel.SetTracerProvider(be.TracerProvider())
	otel.SetMeterProvider(be.MeterProvider())

	p := &Provider{
		cfg:     cfg,
		backend: be,
		logger:  logger,
		tracer:  be.TracerProvider().Tracer(cfg.ServiceName),
		meter:   be.MeterProvider().Meter(cfg.ServiceName),
	}
	setGlobalProvider(p)
	return p, nil
}

// Backend reports the active backend driver name (e.g. "otlp").
func (p *Provider) Backend() string { return p.cfg.Backend }

// Logger returns the contextual slog logger. Pre-decorated with service
// identity; trace_id / correlation_id are injected on each Handle call.
func (p *Provider) Logger() *slog.Logger { return p.logger }

// Tracer returns the OTel tracer scoped to the service name.
func (p *Provider) Tracer() trace.Tracer { return p.tracer }

// Meter returns the OTel meter scoped to the service name.
func (p *Provider) Meter() metric.Meter { return p.meter }

// Shutdown flushes pending telemetry and tears the backend down.
func (p *Provider) Shutdown(ctx context.Context) error {
	clearGlobalProvider(p)
	return p.backend.Shutdown(ctx)
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
		Backend:        BackendStdout,
		LogLevel:       slog.LevelInfo,
	}
	p, _ := NewProvider(context.Background(), cfg)
	return p
}()
