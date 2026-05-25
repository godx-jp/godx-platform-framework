// Package driver defines the contract that all observability driver
// implementations must satisfy and exposes a process-wide registry so the
// observability module can resolve drivers by name at runtime.
//
// Driver implementations live in sibling packages under
// observability/drivers/. Light, dependency-free drivers (stdout, file,
// stack) are auto-registered by the observability package. Heavy drivers
// (otlp, cloudwatch) require an explicit blank import:
//
//	import _ "github.com/godx-jp/godx-platform-framework/observability/drivers/otlp"
//
// Third-party code may implement [Driver] and register it via [Register] —
// for example to ship an in-house Datadog or New Relic adapter without
// forking the framework.
package driver

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// Driver exposes the three telemetry handles the SDK consumes. Implementations
// must be safe for concurrent use.
type Driver interface {
	LoggerHandler() slog.Handler
	TracerProvider() trace.TracerProvider
	MeterProvider() metric.MeterProvider
	Shutdown(ctx context.Context) error
}

// Constructor builds a Driver from a Spec. Each driver package exposes one
// as `New` and registers it with the package-level registry in init().
type Constructor func(ctx context.Context, s Spec) (Driver, error)
