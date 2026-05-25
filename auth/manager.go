package auth

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"

	adriver "github.com/godx-jp/godx-platform-framework/auth/driver"
)

// Manager owns named auth guards and per-guard credential resolvers.
type Manager struct {
	mu        sync.RWMutex
	guards    map[string]adriver.Guard
	resolvers map[string]CredentialResolver
	def       string
}

func NewManager() *Manager {
	return &Manager{
		guards:    map[string]adriver.Guard{},
		resolvers: map[string]CredentialResolver{},
	}
}

func (m *Manager) AddGuard(name string, g adriver.Guard) error {
	if name == "" || g == nil {
		return fmt.Errorf("auth: AddGuard: name and guard required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.guards[name]; exists {
		return fmt.Errorf("auth: guard %q already registered", name)
	}
	m.guards[name] = g
	if m.def == "" {
		m.def = name
	}
	return nil
}

func (m *Manager) SetDefault(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.guards[name]; !ok {
		return fmt.Errorf("auth: SetDefault(%q): not registered", name)
	}
	m.def = name
	return nil
}

func (m *Manager) Default() adriver.Guard {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.guards[m.def]
}

func (m *Manager) DefaultName() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.def
}

func (m *Manager) Guard(name string) (adriver.Guard, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	g, ok := m.guards[name]
	if !ok {
		return nil, fmt.Errorf("auth: guard %q not registered", name)
	}
	return g, nil
}

// SetResolver binds a credential resolver to a registered guard name.
func (m *Manager) SetResolver(name string, resolve CredentialResolver) error {
	if name == "" || resolve == nil {
		return fmt.Errorf("auth: SetResolver: name and resolver required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.guards[name]; !ok {
		return fmt.Errorf("auth: SetResolver(%q): guard not registered", name)
	}
	m.resolvers[name] = resolve
	return nil
}

// Resolver returns the credential resolver for a guard.
func (m *Manager) Resolver(name string) (CredentialResolver, error) {
	m.mu.RLock()
	resolve, ok := m.resolvers[name]
	m.mu.RUnlock()
	if ok {
		return resolve, nil
	}
	g, err := m.Guard(name)
	if err != nil {
		return nil, err
	}
	return ResolverForGuard(g.Name(), "X-API-Key")
}

func (m *Manager) Names() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]string, 0, len(m.guards))
	for n := range m.guards {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

func (m *Manager) Authenticate(ctx context.Context, req *adriver.CredentialRequest) (*Principal, error) {
	if req == nil {
		return nil, fmt.Errorf("auth: credential request required")
	}
	name := req.Guard
	if name == "" {
		m.mu.RLock()
		name = m.def
		m.mu.RUnlock()
	}
	if name == "" {
		return nil, fmt.Errorf("auth: no default guard")
	}
	g, err := m.Guard(name)
	if err != nil {
		return nil, err
	}
	p, err := g.Authenticate(ctx, req)
	if err != nil {
		return nil, err
	}
	if p != nil && p.Guard == "" {
		p.Guard = name
	}
	return p, nil
}

func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	guards := make([]adriver.Guard, 0, len(m.guards))
	for _, g := range m.guards {
		guards = append(guards, g)
	}
	m.mu.Unlock()
	var errs []error
	for _, g := range guards {
		if err := g.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
