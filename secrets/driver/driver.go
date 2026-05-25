// Package driver is the public contract for secrets backend
// implementations.
package driver

import "context"

// Store is the in-process behaviour of one secrets backend. Every
// method must be safe for concurrent use across goroutines.
type Store interface {
	// Name returns the canonical driver name.
	Name() string

	// Get returns the secret stored under key. Returns ErrNotFound
	// when the key does not exist (callers can errors.Is on it to
	// branch on the missing-key path).
	Get(ctx context.Context, key string) ([]byte, error)

	// Put writes value under key. Backends that do not support
	// writes return ErrReadOnly.
	Put(ctx context.Context, key string, value []byte) error

	// Forget removes key. A missing key is not an error. Read-only
	// backends return ErrReadOnly.
	Forget(ctx context.Context, key string) error

	// List returns the keys visible to this store. Backends that
	// cannot enumerate return ErrListNotSupported with a nil slice.
	List(ctx context.Context) ([]string, error)

	// Shutdown releases backend resources (HTTP clients, network
	// connections). Multiple calls must be safe.
	Shutdown(ctx context.Context) error
}

// Constructor builds a Store from a Spec.
type Constructor func(ctx context.Context, spec Spec) (Store, error)
