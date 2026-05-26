package database

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	ddriver "github.com/godx-jp/godx-platform-framework/database/driver"
)

const (
	EnvDefaultConnection = "DATABASE_DEFAULT_CONNECTION"
	EnvConnections       = "DATABASE_CONNECTIONS"
	EnvWriteConnection   = "DATABASE_WRITE_CONNECTION"
	EnvReadConnections   = "DATABASE_READ_CONNECTIONS"
	EnvReadStrategy      = "DATABASE_READ_STRATEGY"
	EnvSticky            = "DATABASE_STICKY"

	EnvLogQueries      = "DATABASE_LOG_QUERIES"
	EnvLogSlow         = "DATABASE_LOG_SLOW_THRESHOLD"
	EnvLogArgs         = "DATABASE_LOG_ARGS"
	EnvTraceQueries    = "DATABASE_TRACE_QUERIES"
	EnvMetricsEnabled  = "DATABASE_METRICS_ENABLED"
	EnvMetricsInterval = "DATABASE_METRICS_INTERVAL"

	envConnPrefix = "DATABASE_CONNECTION_"
)

// ReadStrategy selects a read replica.
type ReadStrategy string

const (
	ReadRoundRobin ReadStrategy = "round_robin"
	ReadRandom     ReadStrategy = "random"
)

// ConnectionConfig configures one named connection.
type ConnectionConfig struct {
	Driver string
	URL    string

	MaxConns          int32
	MinConns          int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
}

// Config is the database module configuration.
type Config struct {
	DefaultConnection string
	Connections       map[string]ConnectionConfig

	WriteConnection string
	ReadConnections []string
	ReadStrategy    ReadStrategy
	Sticky          bool

	LogQueries      bool
	LogSlow         time.Duration
	LogArgs         bool
	TraceQueries    bool
	MetricsEnabled  bool
	MetricsInterval time.Duration
}

// LoadConfigFromEnv builds Config from the environment.
func LoadConfigFromEnv() Config {
	defaultConn := strings.TrimSpace(os.Getenv(EnvDefaultConnection))
	if defaultConn == "" {
		defaultConn = "default"
	}
	names := splitCSV(os.Getenv(EnvConnections))
	if len(names) == 0 {
		names = []string{defaultConn}
	}
	conns := make(map[string]ConnectionConfig, len(names))
	for _, name := range names {
		conns[name] = LoadConnectionConfigFromEnv(name)
	}

	write := strings.TrimSpace(os.Getenv(EnvWriteConnection))
	if write == "" {
		write = defaultConn
	}

	sticky := true
	if v := strings.TrimSpace(os.Getenv(EnvSticky)); v != "" {
		sticky, _ = strconv.ParseBool(v)
	}

	strategy := ReadStrategy(strings.TrimSpace(os.Getenv(EnvReadStrategy)))
	if strategy == "" {
		strategy = ReadRoundRobin
	}

	logQueries, _ := strconv.ParseBool(os.Getenv(EnvLogQueries))
	traceQueries, _ := strconv.ParseBool(os.Getenv(EnvTraceQueries))
	logArgs, _ := strconv.ParseBool(os.Getenv(EnvLogArgs))
	metricsEnabled, _ := strconv.ParseBool(os.Getenv(EnvMetricsEnabled))
	if os.Getenv(EnvMetricsEnabled) == "" {
		metricsEnabled = true
	}
	slow, _ := time.ParseDuration(strings.TrimSpace(os.Getenv(EnvLogSlow)))
	metricsInterval, _ := time.ParseDuration(strings.TrimSpace(os.Getenv(EnvMetricsInterval)))
	if metricsInterval <= 0 {
		metricsInterval = 15 * time.Second
	}

	return Config{
		DefaultConnection: defaultConn,
		Connections:       conns,
		WriteConnection:   write,
		ReadConnections:   splitCSV(os.Getenv(EnvReadConnections)),
		ReadStrategy:      strategy,
		Sticky:            sticky,
		LogQueries:        logQueries,
		LogSlow:           slow,
		LogArgs:           logArgs,
		TraceQueries:      traceQueries,
		MetricsEnabled:    metricsEnabled,
		MetricsInterval:   metricsInterval,
	}
}

// LoadConnectionConfigFromEnv reads DATABASE_CONNECTION_<NAME>_* vars.
func LoadConnectionConfigFromEnv(name string) ConnectionConfig {
	seg := envSegment(name)
	get := func(suffix string) string {
		return strings.TrimSpace(os.Getenv(envConnPrefix + seg + "_" + suffix))
	}
	drv := get("DRIVER")
	if drv == "" {
		switch name {
		case ddriver.DriverPostgres, ddriver.DriverMySQL, ddriver.DriverSQLite:
			drv = name
		default:
			drv = ddriver.DriverPostgres
		}
	}
	maxConns, _ := strconv.ParseInt(get("MAX_CONNS"), 10, 32)
	minConns, _ := strconv.ParseInt(get("MIN_CONNS"), 10, 32)
	maxLife, _ := time.ParseDuration(get("MAX_CONN_LIFETIME"))
	maxIdle, _ := time.ParseDuration(get("MAX_CONN_IDLE_TIME"))
	health, _ := time.ParseDuration(get("HEALTH_CHECK_PERIOD"))

	url := get("URL")
	if url == "" {
		url = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}

	return ConnectionConfig{
		Driver:            drv,
		URL:               url,
		MaxConns:          int32(maxConns),
		MinConns:          int32(minConns),
		MaxConnLifetime:   maxLife,
		MaxConnIdleTime:   maxIdle,
		HealthCheckPeriod: health,
	}
}

func (c Config) Validate() error {
	if c.DefaultConnection == "" {
		return fmt.Errorf("database: default connection is required")
	}
	if len(c.Connections) == 0 {
		return fmt.Errorf("database: no connections configured")
	}
	if _, ok := c.Connections[c.DefaultConnection]; !ok {
		return fmt.Errorf("database: default connection %q not in Connections", c.DefaultConnection)
	}
	if _, ok := c.Connections[c.WriteConnection]; !ok {
		return fmt.Errorf("database: write connection %q not in Connections", c.WriteConnection)
	}
	for _, rc := range c.ReadConnections {
		if _, ok := c.Connections[rc]; !ok {
			return fmt.Errorf("database: read connection %q not in Connections", rc)
		}
	}
	for name, cc := range c.Connections {
		if err := cc.Validate(name); err != nil {
			return err
		}
	}
	return nil
}

func (cc ConnectionConfig) Validate(name string) error {
	if strings.TrimSpace(cc.Driver) == "" {
		return fmt.Errorf("database: connection %q: driver is required", name)
	}
	if strings.TrimSpace(cc.URL) == "" {
		return fmt.Errorf("database: connection %q: URL is required", name)
	}
	return nil
}

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func envSegment(name string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(name) {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		case r == '-':
			b.WriteByte('_')
		}
	}
	return b.String()
}
