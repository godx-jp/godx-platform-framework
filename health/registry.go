package health

import (
	"context"
	"sync"
)

// Probe checks one dependency. Returning nil means healthy.
type Probe func(ctx context.Context) error

type namedProbe struct {
	name string
	fn   Probe
}

// Registry holds registered readiness probes. Liveness (/healthz) reports
// process-up by default; readiness (/readyz) runs every registered probe.
type Registry struct {
	mu    sync.RWMutex
	ready []namedProbe
}

// NewRegistry returns an empty probe registry.
func NewRegistry() *Registry {
	return &Registry{}
}

// RegisterProbe adds a readiness probe. Duplicate names overwrite prior probes.
func (r *Registry) RegisterProbe(name string, p Probe) {
	if name == "" || p == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, np := range r.ready {
		if np.name == name {
			r.ready[i] = namedProbe{name: name, fn: p}
			return
		}
	}
	r.ready = append(r.ready, namedProbe{name: name, fn: p})
}

// Probes returns the sorted names of registered readiness probes.
func (r *Registry) Probes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.ready))
	for _, np := range r.ready {
		out = append(out, np.name)
	}
	sortStrings(out)
	return out
}

func sortStrings(s []string) {
	for i := 0; i < len(s); i++ {
		for j := i + 1; j < len(s); j++ {
			if s[j] < s[i] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}

// CheckReady runs all readiness probes and returns failing probe names.
func (r *Registry) CheckReady(ctx context.Context) map[string]error {
	r.mu.RLock()
	probes := append([]namedProbe(nil), r.ready...)
	r.mu.RUnlock()

	failures := make(map[string]error)
	for _, np := range probes {
		if err := np.fn(ctx); err != nil {
			failures[np.name] = err
		}
	}
	return failures
}
