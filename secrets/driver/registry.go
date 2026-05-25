package driver

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// Canonical driver names.
const (
	DriverEnv   = "env"
	DriverFile  = "file"
	DriverVault = "vault"
	DriverGCPSM = "gcpsm"
	DriverAWSSM = "awssm"
)

var (
	regMu sync.RWMutex
	reg   = map[string]Constructor{}
)

// Register adds a Constructor under name.
func Register(name string, c Constructor) {
	if name == "" {
		panic("secrets: Register called with empty driver name")
	}
	if c == nil {
		panic("secrets: Register called with nil Constructor for " + name)
	}
	regMu.Lock()
	defer regMu.Unlock()
	reg[name] = c
}

// Lookup returns the registered Constructor for name, or nil.
func Lookup(name string) Constructor {
	regMu.RLock()
	defer regMu.RUnlock()
	return reg[name]
}

// Names returns the sorted list of registered driver names.
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

// New constructs a Store from spec by looking up spec.Name.
func New(ctx context.Context, spec Spec) (Store, error) {
	if spec.Name == "" {
		return nil, fmt.Errorf("secrets: driver name is required")
	}
	c := Lookup(spec.Name)
	if c == nil {
		return nil, fmt.Errorf(
			"secrets: driver %q not registered — did you forget to import "+
				"github.com/godx-jp/godx-platform-framework/secrets/drivers/%s ?",
			spec.Name, spec.Name)
	}
	return c(ctx, spec)
}
