package driver

import "time"

const (
	DriverMemory = "memory"
	DriverRedis  = "redis"
)

// Spec configures a ratelimit driver.
type Spec struct {
	Name string

	// Rate is the sustained refill rate (tokens per second).
	Rate float64

	// Burst is the maximum bucket capacity.
	Burst int

	// Prefix scopes Redis keys (memory driver ignores it).
	Prefix string

	// ── redis driver ─────────────────────────────────────────────
	// URL is a full redis URL — redis://[user:pass@]host:port[/db].
	URL string
	// Address is host:port when URL is empty.
	Address string
	Username string
	Password string
	DB       int

	// TTL caps idle bucket keys on Redis (memory: idle eviction interval).
	TTL time.Duration

	Extra map[string]string
}
