// Package driver is the public contract for cache backend
// implementations.
//
// A driver implements the smallest sound set of operations needed to
// build a Laravel-style Cache facade. The user-facing wrapper
// (cache.Store) layers higher-level conveniences (Remember, Pull,
// JSON helpers, etc.) on top of these primitives, so driver
// implementations stay simple and uniform.
//
// All bytes are opaque to the driver — encoding/decoding (JSON,
// gob, msgpack) is the caller's responsibility. The integer
// Increment/Decrement contract is the one exception: implementations
// must treat stored counter values as ASCII decimal integers (matching
// Redis INCRBY semantics) so the operation interoperates across
// drivers.
package driver

import (
	"context"
	"time"
)

// Driver is the in-process behaviour of a single cache store. Every
// method must be safe for concurrent use across goroutines.
type Driver interface {
	// Get returns the value stored under key.
	//   ok == false  ⇒ key absent or expired (val is nil; err is nil)
	//   ok == true   ⇒ key present and unexpired
	//   err  != nil  ⇒ backend failure
	Get(ctx context.Context, key string) (val []byte, ok bool, err error)

	// Put writes value with an optional TTL. ttl == 0 means store
	// indefinitely (matches Laravel's Cache::forever).
	Put(ctx context.Context, key string, val []byte, ttl time.Duration) error

	// Add writes value only if key does not already exist (or has
	// expired). Returns true if the value was stored. Atomic where the
	// backend supports it (Redis SET NX); emulated under a lock for
	// in-process drivers.
	Add(ctx context.Context, key string, val []byte, ttl time.Duration) (bool, error)

	// Forget removes key. A missing key is not an error.
	Forget(ctx context.Context, key string) error

	// Has reports whether key exists and is unexpired.
	Has(ctx context.Context, key string) (bool, error)

	// Flush removes every key managed by this driver. Scoped to the
	// store's key prefix where the driver supports prefixed flush
	// (Redis SCAN+DEL); falls back to a full purge for backends that
	// don't (memory/file). Documented per driver.
	Flush(ctx context.Context) error

	// Increment / Decrement adjust an integer counter atomically.
	// Returns the new value. The key must either be absent or hold a
	// value that decodes as a decimal integer; anything else returns
	// ErrNotInteger. When the key is absent, the counter starts at 0
	// before the delta is applied.
	Increment(ctx context.Context, key string, delta int64) (int64, error)
	Decrement(ctx context.Context, key string, delta int64) (int64, error)

	// Shutdown releases backend resources (open files, network
	// connections). Multiple calls must be safe.
	Shutdown(ctx context.Context) error
}

// Constructor builds a Driver from a Spec. Each driver package exports
// a constructor and registers it via Register at init time.
type Constructor func(ctx context.Context, spec Spec) (Driver, error)
