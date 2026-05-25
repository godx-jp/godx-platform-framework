package driver

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

var (
	regMu sync.RWMutex
	reg   = map[string]Constructor{}
)

func Register(name string, c Constructor) {
	if name == "" || c == nil {
		panic("ratelimit: Register called with empty name or nil constructor")
	}
	regMu.Lock()
	reg[name] = c
	regMu.Unlock()
}

func Lookup(name string) Constructor {
	regMu.RLock()
	defer regMu.RUnlock()
	return reg[name]
}

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

func New(ctx context.Context, spec Spec) (Limiter, error) {
	if spec.Name == "" {
		return nil, fmt.Errorf("ratelimit: driver name is required")
	}
	c := Lookup(spec.Name)
	if c == nil {
		return nil, fmt.Errorf("ratelimit: driver %q not registered — blank-import github.com/godx-jp/godx-platform-framework/ratelimit/drivers/%s", spec.Name, spec.Name)
	}
	return c(ctx, spec)
}
