// Package backends contains pluggable drivers for the observability module.
//
// A [Backend] supplies an [slog.Handler] for logs and OTel
// [trace.TracerProvider] / [metric.MeterProvider] for traces and metrics. The
// observability package builds high-level wrappers on top of these.
//
// Selecting a backend is a deployment-time decision driven by the
// OBS_BACKEND env var (see ../config.go). The "stdout" driver has no
// external dependencies; others require backend-specific endpoints or
// credentials.
package backends

import (
	"context"
	"fmt"
	"log/slog"

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// Backend is a driver that exposes the three concrete handles the SDK needs.
// Implementations must be safe for concurrent use.
type Backend interface {
	LoggerHandler() slog.Handler
	TracerProvider() trace.TracerProvider
	MeterProvider() metric.MeterProvider
	Shutdown(ctx context.Context) error
}

// Spec is the construction input passed by the observability package.
// Backend-specific fields are present even when the active backend ignores
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

	AWSRegion    string
	LogGroupName string
}

// New constructs the named backend.
func New(ctx context.Context, s Spec) (Backend, error) {
	switch s.Name {
	case "", "stdout":
		return newStdout(s), nil
	case "otlp":
		return newOTLP(ctx, s)
	case "cloudwatch":
		return newCloudWatch(ctx, s)
	default:
		return nil, fmt.Errorf("backends: unknown driver %q", s.Name)
	}
}
