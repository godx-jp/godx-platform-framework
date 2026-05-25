// Package driver defines the contract every storage driver implementation
// must satisfy and exposes a process-wide registry so the storage manager
// can resolve drivers by name at runtime.
//
// Driver implementations live in sibling packages under storage/drivers/.
// Light, dependency-free drivers (local, memory) are auto-registered when
// the parent storage package is imported. Heavy drivers (s3, gcs, azure,
// minio) carry cloud-SDK dependencies and require an explicit blank
// import to register, keeping consumers' binaries lean.
//
// The Driver interface intentionally exposes only primitive operations.
// The Laravel-style ergonomic API (Put/Get/Copy/Move/Files/Directories
// /URL/...) is implemented once on storage.Disk on top of these
// primitives, so every driver inherits the full surface for free.
//
// This package mirrors observability/driver — see docs/DRIVER_PATTERN.md
// for the shared convention.
package driver

import (
	"context"
	"io"
	"time"
)

// Driver is the minimal interface every storage backend implements.
//
// Implementations MUST be safe for concurrent use by multiple goroutines.
// Operations that have no meaning for a given backend (URL/SignedURL on
// the memory driver, for example) must return ErrNotSupported rather
// than panicking or silently succeeding.
type Driver interface {
	// NewReader opens key for streaming reads. The caller is responsible
	// for calling Close on the returned reader. Returns ErrNotFound when
	// the key does not exist.
	NewReader(ctx context.Context, key string) (io.ReadCloser, error)

	// NewWriter opens key for streaming writes. The caller commits the
	// write by calling Close on the returned writer. Closing without
	// writing creates an empty object at key.
	NewWriter(ctx context.Context, key string, opts WriteOptions) (io.WriteCloser, error)

	// Delete removes key. Returns ErrNotFound when the key is absent.
	Delete(ctx context.Context, key string) error

	// Exists reports whether key is present.
	Exists(ctx context.Context, key string) (bool, error)

	// Attributes returns metadata for key.
	Attributes(ctx context.Context, key string) (Attributes, error)

	// List returns the direct children of prefix (non-recursive). Each
	// entry is either an object or a synthetic directory (object stores
	// emulate directories by trailing "/").
	List(ctx context.Context, prefix string) ([]Entry, error)

	// URL returns a stable public URL for key. Drivers without a notion
	// of a public URL must return ErrNotSupported.
	URL(key string) (string, error)

	// SignedURL returns a time-limited URL granting read access. Drivers
	// without signed-URL support must return ErrNotSupported.
	SignedURL(ctx context.Context, key string, expires time.Duration) (string, error)

	// Shutdown releases resources held by the driver. It MUST be safe to
	// call more than once.
	Shutdown(ctx context.Context) error
}

// Constructor builds a Driver from a Spec. Every implementation
// registers exactly one Constructor under its driver name via Register.
type Constructor func(ctx context.Context, s Spec) (Driver, error)
