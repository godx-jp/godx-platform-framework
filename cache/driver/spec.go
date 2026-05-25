package driver

import "time"

// Spec is the uniform input to every driver constructor. Mirrors the
// shape used by the storage module so cache stores configure the same
// way (env-driven per-store) and individual driver packages stay free
// of cross-driver coupling.
type Spec struct {
	// Name is the driver name (memory, file, redis). The framework
	// dispatches Constructor lookups by this field; drivers themselves
	// can use it for diagnostics.
	Name string

	// Prefix is prepended to every key handed to the driver. Manager
	// composes the per-store prefix with the global cache prefix
	// before delegating, so driver implementations can apply it
	// directly without further composition.
	Prefix string

	// DefaultTTL is applied by the Store wrapper when the caller passes
	// ttl <= 0 to a Put/Add through a code path that opts in. Drivers
	// themselves treat ttl == 0 as "forever" — they do not consult
	// DefaultTTL.
	DefaultTTL time.Duration

	// ── file driver ──────────────────────────────────────────────
	// Path is the on-disk root directory for the file driver. Required.
	Path string

	// ── redis driver ─────────────────────────────────────────────
	// URL takes precedence when set ("redis://user:pass@host:6379/0").
	URL string
	// Address is host:port (or "unix:///path/to/sock") when URL is
	// empty. Required for the redis driver unless URL is supplied.
	Address  string
	Username string
	Password string
	DB       int
	TLS      bool

	// Extra carries driver-specific extension config so adding a new
	// driver does not require modifying this struct.
	Extra map[string]string
}
