package scheduler

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Environment variable names used by the scheduler module.
const (
	EnvEnabled  = "SCHEDULER_ENABLED"
	EnvLockTTL  = "SCHEDULER_LOCK_TTL"
	EnvLockPref = "SCHEDULER_LOCK_PREFIX"
)

// Config configures the scheduler module.
type Config struct {
	// Enabled starts the cron runner on Init when true. Defaults to true.
	Enabled bool
	// LockTTL is the TTL for OnOneServer distributed locks. Defaults to 24h.
	LockTTL time.Duration
	// LockPrefix prefixes cache lock keys. Defaults to "schedule-lock:".
	LockPrefix string
}

// LoadConfigFromEnv builds a Config from the process environment.
func LoadConfigFromEnv() Config {
	enabled := true
	if v := strings.TrimSpace(os.Getenv(EnvEnabled)); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			enabled = b
		}
	}
	ttl := 24 * time.Hour
	if v := strings.TrimSpace(os.Getenv(EnvLockTTL)); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			ttl = d
		}
	}
	prefix := strings.TrimSpace(os.Getenv(EnvLockPref))
	if prefix == "" {
		prefix = "schedule-lock:"
	}
	return Config{
		Enabled:    enabled,
		LockTTL:    ttl,
		LockPrefix: prefix,
	}
}

func (c Config) withDefaults() Config {
	if c.LockTTL <= 0 {
		c.LockTTL = 24 * time.Hour
	}
	if c.LockPrefix == "" {
		c.LockPrefix = "schedule-lock:"
	}
	return c
}

func (c Config) Validate() error {
	c = c.withDefaults()
	if c.LockTTL <= 0 {
		return fmt.Errorf("scheduler: LockTTL must be positive")
	}
	return nil
}
