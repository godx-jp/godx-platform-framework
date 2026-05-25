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
	BackendFile       = "file"
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

	// File-driver fields. Used when Backend == "file" — Laravel-style
	// local file logging.
	FilePath       string // absolute or relative path; parent dir auto-created
	FileRotate     string // "none" | "daily" | "size"; default "daily"
	FileMaxSizeMB  int    // size-rotation threshold; default 100
	FileMaxAgeDays int    // delete rotated files older than N days; 0 = forever
	FileMaxBackups int    // keep at most N rotated files; 0 = unlimited
	FileCompress   bool   // gzip rotated files
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
//	OBS_LOG_FILE                 (empty)
//	OBS_LOG_ROTATE               daily
//	OBS_LOG_MAX_SIZE_MB          100
//	OBS_LOG_MAX_AGE_DAYS         14
//	OBS_LOG_MAX_BACKUPS          0
//	OBS_LOG_COMPRESS             true
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
		FilePath:        os.Getenv("OBS_LOG_FILE"),
		FileRotate:      getEnv("OBS_LOG_ROTATE", "daily"),
		FileMaxSizeMB:   parseInt(getEnv("OBS_LOG_MAX_SIZE_MB", "100"), 100),
		FileMaxAgeDays:  parseInt(getEnv("OBS_LOG_MAX_AGE_DAYS", "14"), 14),
		FileMaxBackups:  parseInt(getEnv("OBS_LOG_MAX_BACKUPS", "0"), 0),
		FileCompress:    parseBool(getEnv("OBS_LOG_COMPRESS", "true"), true),
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
	case BackendStdout, BackendFile, BackendOTLP, BackendCloudWatch:
	default:
		return fmt.Errorf("observability: unknown backend %q (valid: stdout, file, otlp, cloudwatch)", c.Backend)
	}
	if c.Backend == BackendOTLP && c.OTLPEndpoint == "" {
		return fmt.Errorf("observability: OTLPEndpoint required when backend=otlp (set OTEL_EXPORTER_OTLP_ENDPOINT)")
	}
	if c.Backend == BackendFile && c.FilePath == "" {
		return fmt.Errorf("observability: FilePath required when backend=file (set OBS_LOG_FILE)")
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
