package drivers

import (
	"context"
	"errors"
	"log/slog"

	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

// ErrCloudWatchNotImplemented is returned when the cloudwatch driver is
// selected before its 0.3.0 release.
var ErrCloudWatchNotImplemented = errors.New(
	"drivers: cloudwatch driver is a stub in v0.2.x — pin >=v0.3.0 once the AWS ADOT integration lands",
)

// cloudWatchDriver is a placeholder so OBSERVABILITY_DRIVER=cloudwatch fails
// fast with an actionable error instead of a generic "unknown driver".
type cloudWatchDriver struct{}

func newCloudWatch(_ context.Context, _ Spec) (Driver, error) {
	return nil, ErrCloudWatchNotImplemented
}

// The methods below are unreachable in v0.2.x — they exist so the type
// satisfies [Driver] for future versions where the constructor returns a
// concrete instance.
func (cloudWatchDriver) LoggerHandler() slog.Handler {
	return slog.NewJSONHandler(devnull{}, nil)
}
func (cloudWatchDriver) TracerProvider() trace.TracerProvider { return tracenoop.NewTracerProvider() }
func (cloudWatchDriver) MeterProvider() metric.MeterProvider  { return metricnoop.NewMeterProvider() }
func (cloudWatchDriver) Shutdown(_ context.Context) error     { return nil }

type devnull struct{}

func (devnull) Write(p []byte) (int, error) { return len(p), nil }
