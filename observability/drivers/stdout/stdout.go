// Package stdout is the default observability driver: structured JSON logs
// to stdout, an in-process tracer with no exporter, and a no-op meter. Safe
// in any environment because it has no external dependencies. Registered
// automatically when the observability package is imported.
package stdout

import (
	"context"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"github.com/godx-jp/godx-platform-framework/observability/driver"
)

// Name is the identifier used by OBSERVABILITY_DRIVER to select this driver.
const Name = "stdout"

func init() { driver.Register(Name, New) }

// New constructs the stdout driver. Exported so callers can build the driver
// outside the registry (tests, embedding) without dancing around blank imports.
func New(_ context.Context, s driver.Spec) (driver.Driver, error) {
	return &impl{
		handler: slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: s.LogLevel}),
		tp: sdktrace.NewTracerProvider(
			sdktrace.WithSampler(driver.SamplerFor(s.TraceSampleRate)),
			sdktrace.WithResource(driver.ResourceFor(s)),
		),
	}, nil
}

type impl struct {
	handler slog.Handler
	tp      *sdktrace.TracerProvider
}

func (d *impl) LoggerHandler() slog.Handler          { return d.handler }
func (d *impl) TracerProvider() trace.TracerProvider { return d.tp }
func (d *impl) MeterProvider() metric.MeterProvider  { return metricnoop.NewMeterProvider() }
func (d *impl) Shutdown(ctx context.Context) error   { return d.tp.Shutdown(ctx) }
