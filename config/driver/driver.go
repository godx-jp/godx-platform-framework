// Package driver is the public contract for config Source implementations.
//
// Sources produce a tree of values (map[string]any with strings,
// numbers, booleans, slices, and nested maps). Repository merges
// every Source's tree in registration order — later sources override
// earlier ones. Sources never own typed access; that's Repository's
// job, so adding a new backend means writing one Load method plus an
// optional Watch.
package driver

import (
	"context"
)

// Source is the in-process behaviour of one configuration backend.
// Implementations must be safe for concurrent calls to Load.
type Source interface {
	// Name returns the canonical driver name (env, file, …). Used by
	// the registry, diagnostics, and error messages.
	Name() string

	// Load returns the current tree. Implementations should perform
	// I/O each call so callers can rebuild a Repository on demand
	// without ferrying state through other channels. Returning an
	// empty map is valid — the merge layer treats it as "nothing".
	Load(ctx context.Context) (map[string]any, error)

	// Shutdown releases backend resources (file handles, network
	// connections, watcher goroutines). Multiple calls must be safe.
	Shutdown(ctx context.Context) error
}

// Watcher is an optional capability for Sources that can stream
// change notifications. Drivers that do not support watching simply
// do not implement this interface — Repository will still rebuild on
// demand via Repository.Reload.
type Watcher interface {
	// Watch invokes onChange whenever the source's tree changes. The
	// callback should be cheap; consumers that need expensive work
	// should fan out to a goroutine. Watch must return promptly
	// after launching its background goroutine; Shutdown stops it.
	Watch(ctx context.Context, onChange func()) error
}

// Constructor builds a Source from a Spec. Each driver package
// exports one and registers it at init time.
type Constructor func(ctx context.Context, spec Spec) (Source, error)
