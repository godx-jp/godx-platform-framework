package driver

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

var registry = struct {
	mu sync.RWMutex
	m  map[string]Constructor
}{m: make(map[string]Constructor)}

// Register adds a driver constructor under the given name. Intended for use
// from init() in driver packages; calls after init() are also legal but
// must coordinate with whatever startup code reads the registry.
//
// Re-registering the same name overwrites the prior entry. This is
// intentional — projects sometimes need to replace a built-in driver with a
// patched version without forking the framework.
func Register(name string, ctor Constructor) {
	if name == "" {
		panic("observability/driver: Register with empty name")
	}
	if ctor == nil {
		panic("observability/driver: Register with nil constructor")
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.m[name] = ctor
}

// Lookup returns the constructor registered under name and a boolean
// indicating whether it was found.
func Lookup(name string) (Constructor, bool) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	ctor, ok := registry.m[name]
	return ctor, ok
}

// Names returns every registered driver name in sorted order. Useful for
// diagnostics, CLI help, and the not-found error message.
func Names() []string {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	out := make([]string, 0, len(registry.m))
	for k := range registry.m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// New is the single entry point the observability package uses to build a
// driver. Returns a clear error when name has not been registered —
// typically because the user forgot to blank-import the corresponding driver
// package.
func New(ctx context.Context, s Spec) (Driver, error) {
	ctor, ok := Lookup(s.Name)
	if !ok {
		return nil, fmt.Errorf(
			"observability/driver: %q not registered (have: %v) — "+
				"heavy drivers require an explicit blank import, e.g. "+
				`_ "github.com/godx-jp/godx-platform-framework/observability/drivers/otlp"`,
			s.Name, Names(),
		)
	}
	return ctor(ctx, s)
}
