package backends

import (
	"context"
	"log/slog"
	"os"

	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
)

// stdoutBackend writes structured JSON logs to stdout and uses a no-op meter
// + an in-process tracer (always-sample). Suitable for development and unit
// tests where no external collector is reachable.
type stdoutBackend struct {
	handler slog.Handler
	tp      *sdktrace.TracerProvider
}

func newStdout(s Spec) *stdoutBackend {
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: s.LogLevel})
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(samplerFor(s.TraceSampleRate)),
		sdktrace.WithResource(resourceFor(s)),
	)
	return &stdoutBackend{handler: h, tp: tp}
}

func (b *stdoutBackend) LoggerHandler() slog.Handler          { return b.handler }
func (b *stdoutBackend) TracerProvider() trace.TracerProvider { return b.tp }
func (b *stdoutBackend) MeterProvider() metric.MeterProvider  { return metricnoop.NewMeterProvider() }
func (b *stdoutBackend) Shutdown(ctx context.Context) error   { return b.tp.Shutdown(ctx) }

func samplerFor(rate float64) sdktrace.Sampler {
	if rate <= 0 || rate >= 1 {
		return sdktrace.AlwaysSample()
	}
	return sdktrace.ParentBased(sdktrace.TraceIDRatioBased(rate))
}

func resourceFor(s Spec) *sdkresource.Resource {
	attrs := []sdkresource.Option{
		sdkresource.WithAttributes(
			semconv.ServiceName(s.ServiceName),
			semconv.ServiceVersion(s.ServiceVersion),
			semconv.DeploymentEnvironment(s.Environment),
		),
	}
	r, _ := sdkresource.New(context.Background(), attrs...)
	return r
}
