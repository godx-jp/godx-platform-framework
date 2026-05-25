// Package cloudwatch is a stub driver that fails fast when AWS CloudWatch
// is selected before the AWS ADOT integration ships (>= v0.6.0). Designed
// as opt-in from day one so the future heavy-dependency version stays
// backwards-compatible with how callers wire it up.
//
//	import _ "github.com/godx-jp/godx-platform-framework/observability/drivers/cloudwatch"
package cloudwatch

import (
	"context"
	"errors"
	"log/slog"

	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"

	"github.com/godx-jp/godx-platform-framework/observability/driver"
)

// Name is the identifier used by OBSERVABILITY_DRIVER to select this driver.
const Name = "cloudwatch"

// ErrNotImplemented is returned when this driver is selected before the AWS
// ADOT integration lands. Pin >= v0.6.0 once the constructor returns a real
// instance.
var ErrNotImplemented = errors.New(
	"observability/drivers/cloudwatch: stub in v0.4.x–v0.5.x — pin >= v0.6.0 once the AWS ADOT integration lands",
)

func init() { driver.Register(Name, New) }

// New is a no-op constructor in v0.4.x; always returns [ErrNotImplemented].
func New(_ context.Context, _ driver.Spec) (driver.Driver, error) {
	return nil, ErrNotImplemented
}

// stub is kept so the type can satisfy [driver.Driver] in future versions
// without a follow-up rename.
type stub struct{}

func (stub) LoggerHandler() slog.Handler          { return slog.NewJSONHandler(devnull{}, nil) }
func (stub) TracerProvider() trace.TracerProvider { return tracenoop.NewTracerProvider() }
func (stub) MeterProvider() metric.MeterProvider  { return metricnoop.NewMeterProvider() }
func (stub) Shutdown(_ context.Context) error     { return nil }

type devnull struct{}

func (devnull) Write(p []byte) (int, error) { return len(p), nil }
