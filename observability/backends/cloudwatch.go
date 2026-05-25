package backends

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
// selected before its 0.2.0 release.
var ErrCloudWatchNotImplemented = errors.New(
	"backends: cloudwatch driver is a stub in v0.1.0 — pin >=v0.2.0 once the AWS ADOT integration lands",
)

// cloudWatchBackend is a placeholder so OBS_BACKEND=cloudwatch fails fast
// with an actionable error instead of a generic "unknown driver".
type cloudWatchBackend struct{}

func newCloudWatch(_ context.Context, _ Spec) (Backend, error) {
	return nil, ErrCloudWatchNotImplemented
}

// The methods below are unreachable in v0.1.0 — they exist so the type
// satisfies [Backend] for future versions where the constructor returns a
// concrete instance.
func (cloudWatchBackend) LoggerHandler() slog.Handler {
	return slog.NewJSONHandler(devnull{}, nil)
}
func (cloudWatchBackend) TracerProvider() trace.TracerProvider { return tracenoop.NewTracerProvider() }
func (cloudWatchBackend) MeterProvider() metric.MeterProvider  { return metricnoop.NewMeterProvider() }
func (cloudWatchBackend) Shutdown(_ context.Context) error     { return nil }

type devnull struct{}

func (devnull) Write(p []byte) (int, error) { return len(p), nil }
