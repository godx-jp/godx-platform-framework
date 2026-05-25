package featureflag

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	fdriver "github.com/godx-jp/godx-platform-framework/featureflag/driver"
)

const (
	EnvDriver    = "FEATUREFLAG_DRIVER"
	EnvPrefix    = "FEATUREFLAG_PREFIX"
	EnvCache     = "FEATUREFLAG_CACHE"
	EnvCacheTTL  = "FEATUREFLAG_CACHE_TTL"
)

// Config configures the featureflag module.
type Config struct {
	Driver   string
	Prefix   string
	Cache    bool
	CacheTTL time.Duration

	// Heavy driver knobs (env-driven).
	SDKKey   string
	Endpoint string
	Project  string
	AppName  string
}

// LoadConfigFromEnv builds Config from the process environment.
func LoadConfigFromEnv() Config {
	driver := strings.TrimSpace(os.Getenv(EnvDriver))
	if driver == "" {
		driver = fdriver.DriverConfig
	}
	prefix := strings.TrimSpace(os.Getenv(EnvPrefix))
	if prefix == "" {
		prefix = "flags"
	}
	cache := false
	if v := strings.TrimSpace(os.Getenv(EnvCache)); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			cache = b
		}
	}
	ttl := time.Minute
	if v := strings.TrimSpace(os.Getenv(EnvCacheTTL)); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			ttl = d
		}
	}
	return Config{
		Driver:   driver,
		Prefix:   prefix,
		Cache:    cache,
		CacheTTL: ttl,
		SDKKey:   strings.TrimSpace(os.Getenv("FEATUREFLAG_SDK_KEY")),
		Endpoint: strings.TrimSpace(os.Getenv("FEATUREFLAG_ENDPOINT")),
		Project:  strings.TrimSpace(os.Getenv("FEATUREFLAG_PROJECT")),
		AppName:  strings.TrimSpace(os.Getenv("FEATUREFLAG_APP_NAME")),
	}
}

func (c Config) withDefaults() Config {
	if c.Driver == "" {
		c.Driver = fdriver.DriverConfig
	}
	if c.Prefix == "" {
		c.Prefix = "flags"
	}
	if c.CacheTTL <= 0 {
		c.CacheTTL = time.Minute
	}
	return c
}

func (c Config) Validate() error {
	c = c.withDefaults()
	if c.Driver == "" {
		return fmt.Errorf("featureflag: Driver is required")
	}
	if c.CacheTTL <= 0 {
		return fmt.Errorf("featureflag: CacheTTL must be positive")
	}
	return nil
}
