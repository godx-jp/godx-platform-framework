package observability

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

// Backend identifiers recognised by [LoadConfigFromEnv].
const (
	BackendStdout     = "stdout"
	BackendOTLP       = "otlp"
	BackendCloudWatch = "cloudwatch"
)

// Config controls observability bootstrap. The framework module loads it from
// the environment (see [LoadConfigFromEnv]); callers may construct one
// explicitly for tests or for embedding the SDK outside the framework.
type Config struct {
	// ServiceName / ServiceVersion populate the `service.name` and
	// `service.version` resource attributes. When using the framework
	// module they are taken from [framework.App] automatically.
	ServiceName    string
	ServiceVersion string

	// Environment populates `deployment.environment` (OTel semconv).
	Environment string

	// Backend selects which driver runs. See backends/ for valid values.
	Backend string

	// LogLevel applied to the slog handler.
	LogLevel slog.Level

	// TraceSampleRate in [0,1]. Values outside the range mean AlwaysSample.
	TraceSampleRate float64

	// OTLPEndpoint is the OTLP host:port (no scheme) when Backend == "otlp".
	OTLPEndpoint string
	// OTLPProtocol is "grpc" or "http". Defaults to "grpc".
	OTLPProtocol string
	// OTLPInsecure skips TLS verification (dev only).
	OTLPInsecure bool

	// CloudWatch-only fields. Reserved for the 0.2.0 cloudwatch backend.
	AWSRegion    string
	LogGroupName string
}

// LoadConfigFromEnv reads SDK configuration from environment variables.
//
// Reads, with defaults:
//
//	OBS_BACKEND                  stdout
//	OBS_LOG_LEVEL                info
//	OBS_TRACE_SAMPLE             1.0
//	DEPLOYMENT_ENVIRONMENT       dev
//	OTEL_EXPORTER_OTLP_ENDPOINT  http://localhost:4317
//	OTEL_EXPORTER_OTLP_PROTOCOL  grpc
//	OTEL_EXPORTER_OTLP_INSECURE  true
//	AWS_REGION                   (empty)
//	OBS_LOG_GROUP                /service/{ServiceName}
//
// ServiceName / ServiceVersion are not populated by this function; the
// framework module sets them from [framework.App].
func LoadConfigFromEnv() Config {
	cfg := Config{
		Backend:         getEnv("OBS_BACKEND", BackendStdout),
		LogLevel:        parseLogLevel(getEnv("OBS_LOG_LEVEL", "info")),
		TraceSampleRate: parseFloat(getEnv("OBS_TRACE_SAMPLE", "1.0"), 1.0),
		Environment:     getEnv("DEPLOYMENT_ENVIRONMENT", "dev"),
		OTLPEndpoint:    getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
		OTLPProtocol:    getEnv("OTEL_EXPORTER_OTLP_PROTOCOL", "grpc"),
		OTLPInsecure:    parseBool(getEnv("OTEL_EXPORTER_OTLP_INSECURE", "true"), true),
		AWSRegion:       os.Getenv("AWS_REGION"),
		LogGroupName:    os.Getenv("OBS_LOG_GROUP"),
	}
	return cfg
}

// Validate returns an error if the config is malformed for its selected
// backend. ServiceName must be non-empty.
func (c Config) Validate() error {
	if c.ServiceName == "" {
		return fmt.Errorf("observability: ServiceName is required")
	}
	switch c.Backend {
	case BackendStdout, BackendOTLP, BackendCloudWatch:
	default:
		return fmt.Errorf("observability: unknown backend %q (valid: stdout, otlp, cloudwatch)", c.Backend)
	}
	if c.Backend == BackendOTLP && c.OTLPEndpoint == "" {
		return fmt.Errorf("observability: OTLPEndpoint required when backend=otlp (set OTEL_EXPORTER_OTLP_ENDPOINT)")
	}
	return nil
}

func getEnv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func parseLogLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func parseFloat(s string, def float64) float64 {
	if v, err := strconv.ParseFloat(s, 64); err == nil {
		return v
	}
	return def
}

func parseBool(s string, def bool) bool {
	if v, err := strconv.ParseBool(s); err == nil {
		return v
	}
	return def
}
