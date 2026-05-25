package driver

import "log/slog"

// Spec is the construction input the observability package passes to every
// driver. Driver-specific fields are always present even when the active
// driver ignores them — each implementation should validate the subset it
// actually needs.
//
// Keeping a single uniform Spec (rather than a per-driver Options type) lets
// the registry call every Constructor with the same signature and lets the
// stack driver clone its parent Spec into each sub-driver without losing
// fields the underlying driver needs.
type Spec struct {
	// Name is the driver identifier requested by the user. Constructors
	// can ignore it — the registry has already used it to look them up.
	Name string

	// Service identity (resource attributes service.name, service.version,
	// deployment.environment).
	ServiceName    string
	ServiceVersion string
	Environment    string

	// Log + trace tuning shared by every driver.
	LogLevel        slog.Level
	TraceSampleRate float64

	// OTLP fields (driver=otlp).
	OTLPEndpoint string
	OTLPProtocol string
	OTLPInsecure bool

	// CloudWatch fields (driver=cloudwatch).
	AWSRegion          string
	CloudWatchLogGroup string

	// File fields (driver=file) — Laravel-style local file logging.
	LogFilePath       string // absolute or relative; parent dir auto-created
	LogFileRotation   string // "none" | "daily" (default) | "size"
	LogFileMaxSizeMB  int
	LogFileMaxAgeDays int
	LogFileMaxBackups int
	LogFileCompress   bool

	// Stack fields (driver=stack) — list of sub-driver names that receive
	// every log record. Each sub-driver is built from the same Spec.
	StackDrivers []string
}
