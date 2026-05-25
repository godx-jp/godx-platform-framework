package observability

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

// Driver identifiers recognised by [LoadConfigFromEnv].
//
// The "driver" is the in-process code that talks to a telemetry destination
// ("backend"). Selecting a driver is the only knob you need to swap where
// telemetry goes.
const (
	DriverStdout     = "stdout"     // pretty JSON logs to stdout; dev / containers
	DriverFile       = "file"       // local file with optional rotation; bare-metal / VM
	DriverOTLP       = "otlp"       // OTLP gRPC/HTTP; godx-platform-observability, Datadog, New Relic, …
	DriverCloudWatch = "cloudwatch" // AWS CloudWatch Logs/Metrics + X-Ray (stub in 0.2.x, full in 0.3.0)
)

// Config controls observability bootstrap. The framework module loads it
// from the environment (see [LoadConfigFromEnv]); callers may construct one
// explicitly for tests or for embedding the SDK outside the framework.
type Config struct {
	// ServiceName / ServiceVersion populate the `service.name` and
	// `service.version` resource attributes. When using the framework
	// module they are taken from [framework.App] automatically.
	ServiceName    string
	ServiceVersion string

	// Environment populates `deployment.environment` (OTel semconv).
	Environment string

	// Driver selects which driver runs. See drivers/ for valid values.
	Driver string

	// LogLevel applied to the slog handler.
	LogLevel slog.Level

	// TraceSampleRate in [0,1]. Values outside the range mean AlwaysSample.
	TraceSampleRate float64

	// OTLPEndpoint is the OTLP host:port (no scheme) when Driver == "otlp".
	OTLPEndpoint string
	// OTLPProtocol is "grpc" or "http". Defaults to "grpc".
	OTLPProtocol string
	// OTLPInsecure skips TLS verification (dev only).
	OTLPInsecure bool

	// CloudWatch-only fields. Reserved for the 0.3.0 cloudwatch driver.
	AWSRegion          string
	CloudWatchLogGroup string

	// File-driver fields. Used when Driver == "file" — Laravel-style local
	// file logging.
	LogFilePath       string // absolute or relative path; parent dir auto-created
	LogFileRotation   string // "none" | "daily" | "size"; default "daily"
	LogFileMaxSizeMB  int    // size-rotation threshold; default 100
	LogFileMaxAgeDays int    // delete rotated files older than N days; 0 = forever
	LogFileMaxBackups int    // keep at most N rotated files; 0 = unlimited
	LogFileCompress   bool   // gzip rotated files
}

// LoadConfigFromEnv reads SDK configuration from environment variables.
//
// Reads, with defaults:
//
//	OBSERVABILITY_DRIVER                  stdout
//	OBSERVABILITY_LOG_LEVEL               info
//	OBSERVABILITY_TRACE_SAMPLE_RATE       1.0
//	DEPLOYMENT_ENVIRONMENT                dev
//	OTEL_EXPORTER_OTLP_ENDPOINT           (empty)
//	OTEL_EXPORTER_OTLP_PROTOCOL           grpc
//	OTEL_EXPORTER_OTLP_INSECURE           true
//	AWS_REGION                            (empty)
//	OBSERVABILITY_CLOUDWATCH_LOG_GROUP    (empty)
//	OBSERVABILITY_LOG_FILE_PATH           (empty)
//	OBSERVABILITY_LOG_FILE_ROTATION       daily
//	OBSERVABILITY_LOG_FILE_MAX_SIZE_MB    100
//	OBSERVABILITY_LOG_FILE_MAX_AGE_DAYS   14
//	OBSERVABILITY_LOG_FILE_MAX_BACKUPS    0
//	OBSERVABILITY_LOG_FILE_COMPRESS       true
//
// ServiceName / ServiceVersion are not populated by this function; the
// framework module sets them from [framework.App].
func LoadConfigFromEnv() Config {
	cfg := Config{
		Driver:             getEnv("OBSERVABILITY_DRIVER", DriverStdout),
		LogLevel:           parseLogLevel(getEnv("OBSERVABILITY_LOG_LEVEL", "info")),
		TraceSampleRate:    parseFloat(getEnv("OBSERVABILITY_TRACE_SAMPLE_RATE", "1.0"), 1.0),
		Environment:        getEnv("DEPLOYMENT_ENVIRONMENT", "dev"),
		OTLPEndpoint:       getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
		OTLPProtocol:       getEnv("OTEL_EXPORTER_OTLP_PROTOCOL", "grpc"),
		OTLPInsecure:       parseBool(getEnv("OTEL_EXPORTER_OTLP_INSECURE", "true"), true),
		AWSRegion:          os.Getenv("AWS_REGION"),
		CloudWatchLogGroup: os.Getenv("OBSERVABILITY_CLOUDWATCH_LOG_GROUP"),
		LogFilePath:        os.Getenv("OBSERVABILITY_LOG_FILE_PATH"),
		LogFileRotation:    getEnv("OBSERVABILITY_LOG_FILE_ROTATION", "daily"),
		LogFileMaxSizeMB:   parseInt(getEnv("OBSERVABILITY_LOG_FILE_MAX_SIZE_MB", "100"), 100),
		LogFileMaxAgeDays:  parseInt(getEnv("OBSERVABILITY_LOG_FILE_MAX_AGE_DAYS", "14"), 14),
		LogFileMaxBackups:  parseInt(getEnv("OBSERVABILITY_LOG_FILE_MAX_BACKUPS", "0"), 0),
		LogFileCompress:    parseBool(getEnv("OBSERVABILITY_LOG_FILE_COMPRESS", "true"), true),
	}
	return cfg
}

// Validate returns an error if the config is malformed for its selected
// driver. ServiceName must be non-empty.
func (c Config) Validate() error {
	if c.ServiceName == "" {
		return fmt.Errorf("observability: ServiceName is required")
	}
	switch c.Driver {
	case DriverStdout, DriverFile, DriverOTLP, DriverCloudWatch:
	default:
		return fmt.Errorf("observability: unknown driver %q (valid: stdout, file, otlp, cloudwatch)", c.Driver)
	}
	if c.Driver == DriverOTLP && c.OTLPEndpoint == "" {
		return fmt.Errorf("observability: OTLPEndpoint required when driver=otlp (set OTEL_EXPORTER_OTLP_ENDPOINT)")
	}
	if c.Driver == DriverFile && c.LogFilePath == "" {
		return fmt.Errorf("observability: LogFilePath required when driver=file (set OBSERVABILITY_LOG_FILE_PATH)")
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

func parseInt(s string, def int) int {
	if v, err := strconv.Atoi(s); err == nil {
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
