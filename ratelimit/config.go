package ratelimit

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	rdriver "github.com/godx-jp/godx-platform-framework/ratelimit/driver"
)

const (
	EnvDefault = "RATELIMIT_DEFAULT"
	EnvLimiters = "RATELIMIT_LIMITERS"
	EnvRate    = "RATELIMIT_RATE"
	EnvBurst   = "RATELIMIT_BURST"
	EnvPrefix  = "RATELIMIT_PREFIX"

	envLimiterDriver = "RATELIMIT_LIMITER_%s_DRIVER"
	envLimiterRate   = "RATELIMIT_LIMITER_%s_RATE"
	envLimiterBurst  = "RATELIMIT_LIMITER_%s_BURST"
	envLimiterPrefix = "RATELIMIT_LIMITER_%s_PREFIX"
	envLimiterURL    = "RATELIMIT_LIMITER_%s_URL"
	envLimiterAddr   = "RATELIMIT_LIMITER_%s_ADDRESS"
	envLimiterUser   = "RATELIMIT_LIMITER_%s_USERNAME"
	envLimiterPass   = "RATELIMIT_LIMITER_%s_PASSWORD"
	envLimiterDB     = "RATELIMIT_LIMITER_%s_DB"
)

type LimiterConfig struct {
	Driver string
	Spec   rdriver.Spec
}

type Config struct {
	Default  string
	Limiters map[string]LimiterConfig
}

func LoadConfigFromEnv() Config {
	def := strings.TrimSpace(os.Getenv(EnvDefault))
	if def == "" {
		def = rdriver.DriverMemory
	}
	names := splitCSV(os.Getenv(EnvLimiters))
	if len(names) == 0 {
		names = []string{def}
	}
	globalRate := parseFloat(os.Getenv(EnvRate), 10)
	globalBurst := parseInt(os.Getenv(EnvBurst), 20)
	globalPrefix := os.Getenv(EnvPrefix)

	limiters := make(map[string]LimiterConfig, len(names))
	for _, name := range names {
		limiters[name] = LimiterConfig{
			Driver: inferDriver(name),
			Spec:   loadSpec(name, globalRate, globalBurst, globalPrefix),
		}
	}
	return Config{Default: def, Limiters: limiters}
}

func inferDriver(name string) string {
	key := fmt.Sprintf(envLimiterDriver, envKey(name))
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	switch strings.ToLower(name) {
	case rdriver.DriverMemory, rdriver.DriverRedis:
		return strings.ToLower(name)
	default:
		return name
	}
}

func loadSpec(name string, globalRate float64, globalBurst int, globalPrefix string) rdriver.Spec {
	spec := rdriver.Spec{
		Rate:   globalRate,
		Burst:  globalBurst,
		Prefix: globalPrefix,
	}
	if v := os.Getenv(fmt.Sprintf(envLimiterRate, envKey(name))); v != "" {
		spec.Rate = parseFloat(v, globalRate)
	}
	if v := os.Getenv(fmt.Sprintf(envLimiterBurst, envKey(name))); v != "" {
		spec.Burst = parseInt(v, globalBurst)
	}
	if v := os.Getenv(fmt.Sprintf(envLimiterPrefix, envKey(name))); v != "" {
		spec.Prefix = v
	}
	spec.URL = os.Getenv(fmt.Sprintf(envLimiterURL, envKey(name)))
	spec.Address = os.Getenv(fmt.Sprintf(envLimiterAddr, envKey(name)))
	spec.Username = os.Getenv(fmt.Sprintf(envLimiterUser, envKey(name)))
	spec.Password = os.Getenv(fmt.Sprintf(envLimiterPass, envKey(name)))
	if v := os.Getenv(fmt.Sprintf(envLimiterDB, envKey(name))); v != "" {
		spec.DB = parseInt(v, 0)
	}
	return spec
}

func (c Config) Validate() error {
	if c.Default == "" {
		return fmt.Errorf("ratelimit: default limiter name required")
	}
	if len(c.Limiters) == 0 {
		return fmt.Errorf("ratelimit: no limiters configured")
	}
	if _, ok := c.Limiters[c.Default]; !ok {
		return fmt.Errorf("ratelimit: default %q not in Limiters", c.Default)
	}
	for name, lc := range c.Limiters {
		if strings.TrimSpace(lc.Driver) == "" {
			return fmt.Errorf("ratelimit: limiter %q: driver is required", name)
		}
	}
	return nil
}

func envKey(name string) string {
	return strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
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

func parseFloat(s string, def float64) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f <= 0 {
		return def
	}
	return f
}

func parseInt(s string, def int) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}
