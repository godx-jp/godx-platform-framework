package driver

import (
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const (
	DriverPostgres = "postgres"
	DriverMySQL    = "mysql"
	DriverSQLite   = "sqlite"
)

// Spec is the uniform input to every driver constructor.
type Spec struct {
	ConnectionName string
	Name           string
	URL            string

	MaxConns          int32
	MinConns          int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration

	Obs ObservabilitySpec
}

// ObservabilitySpec controls query logging and tracing.
type ObservabilitySpec struct {
	LogQueries     bool
	SlowThreshold  time.Duration
	LogArgs        bool
	TraceQueries   bool
	MetricsEnabled bool

	Logger         *slog.Logger
	TracerProvider trace.TracerProvider
	MeterProvider  metric.MeterProvider
}
