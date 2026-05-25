package driver

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// Canonical driver names. Centralised so callers (config validation,
// env parsing, tests) reference the same string.
const (
	DriverEnv    = "env"
	DriverFile   = "file"
	DriverRemote = "remote"
	DriverStatic = "static" // in-process map source, mainly for tests
)

var (
	regMu sync.RWMutex
	reg   = map[string]Constructor{}
)

// Register adds a Constructor under name. Driver packages call this
// from init(); calling twice with the same name overwrites the
// previous registration to keep tests deterministic.
func Register(name string, c Constructor) {
	if name == "" {
		panic("config: Register called with empty driver name")
	}
	if c == nil {
		panic("config: Register called with nil Constructor for " + name)
	}
	regMu.Lock()
	defer regMu.Unlock()
	reg[name] = c
}

// Lookup returns the registered Constructor for name, or nil when no
// driver is registered. Use New to receive the standard
// "driver not registered" error.
func Lookup(name string) Constructor {
	regMu.RLock()
	defer regMu.RUnlock()
	return reg[name]
}

// Names returns the sorted list of registered driver names. Useful
// for diagnostics and tests.
func Names() []string {
	regMu.RLock()
	defer regMu.RUnlock()
	out := make([]string, 0, len(reg))
	for n := range reg {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// New constructs a Source from spec by looking up spec.Name in the
// registry. Returns a clear error mentioning the import path the
// caller likely forgot to add when the driver is missing.
func New(ctx context.Context, spec Spec) (Source, error) {
	if spec.Name == "" {
		return nil, fmt.Errorf("config: driver name is required")
	}
	c := Lookup(spec.Name)
	if c == nil {
		return nil, fmt.Errorf(
			"config: driver %q not registered — did you forget to import "+
				"github.com/godx-jp/godx-platform-framework/config/drivers/%s ?",
			spec.Name, spec.Name)
	}
	return c(ctx, spec)
}
