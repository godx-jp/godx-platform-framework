// Package drivers contains pluggable implementations for the observability
// module. A [Driver] is the in-process code that adapts the SDK's standard
// telemetry handles (slog handler, OpenTelemetry tracer and meter providers)
// to a specific destination — stdout, a local file, an OTLP receiver, or a
// hosted product like CloudWatch.
//
// The destination is the "backend"; the driver is the code that talks to it.
// Selecting a driver is a deployment-time decision driven by the
// OBSERVABILITY_DRIVER env var (see ../config.go). The "stdout" and "file"
// drivers require no external infrastructure; others depend on a reachable
// endpoint or cloud credentials.
package drivers

import (
	"context"
	"fmt"
	"log/slog"

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// Driver exposes the three concrete telemetry handles the SDK needs.
// Implementations must be safe for concurrent use.
type Driver interface {
	LoggerHandler() slog.Handler
	TracerProvider() trace.TracerProvider
	MeterProvider() metric.MeterProvider
	Shutdown(ctx context.Context) error
}

// Spec is the construction input passed by the observability package.
// Driver-specific fields are present even when the active driver ignores
// them — implementations should validate the subset they need.
type Spec struct {
	Name string

	ServiceName    string
	ServiceVersion string
	Environment    string

	LogLevel        slog.Level
	TraceSampleRate float64

	OTLPEndpoint string
	OTLPProtocol string
	OTLPInsecure bool

	AWSRegion           string
	CloudWatchLogGroup  string

	// File driver — Laravel-style local file logging with optional
	// rotation. Ignored by other drivers.
	LogFilePath        string
	LogFileRotation    string // "none" | "daily" (default) | "size"
	LogFileMaxSizeMB   int    // size rotation threshold (default 100)
	LogFileMaxAgeDays  int    // delete rotated files older than N days (0 = keep forever)
	LogFileMaxBackups  int    // keep at most N rotated files (0 = unlimited)
	LogFileCompress    bool   // gzip rotated files

	// Stack driver — list of sub-driver names that receive every log
	// record. Each sub-driver is built from the same Spec (so file/OTLP
	// settings flow through). Ignored by non-stack drivers.
	StackDrivers []string
}

// New constructs the named driver.
func New(ctx context.Context, s Spec) (Driver, error) {
	switch s.Name {
	case "", "stdout":
		return newStdout(s), nil
	case "file":
		return newFile(s)
	case "otlp":
		return newOTLP(ctx, s)
	case "cloudwatch":
		return newCloudWatch(ctx, s)
	case "stack":
		return newStack(ctx, s)
	default:
		return nil, fmt.Errorf("drivers: unknown driver %q", s.Name)
	}
}
