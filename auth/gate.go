package auth

import (
	"fmt"
	"net/http"
	"sync"

	adriver "github.com/godx-jp/godx-platform-framework/auth/driver"
)

// GateFunc evaluates authorization for a principal (Laravel-style gate).
type GateFunc func(p *Principal) bool

var (
	gateMu sync.RWMutex
	gates  = map[string]GateFunc{}
)

// Define registers a named gate function. Returns an error if the name is already taken.
func Define(name string, fn GateFunc) error {
	if name == "" || fn == nil {
		return fmt.Errorf("auth: Define: name and function required")
	}
	gateMu.Lock()
	defer gateMu.Unlock()
	if _, exists := gates[name]; exists {
		return fmt.Errorf("auth: gate %q already defined", name)
	}
	gates[name] = fn
	return nil
}

// MustDefine registers a gate and panics on duplicate names.
func MustDefine(name string, fn GateFunc) {
	if err := Define(name, fn); err != nil {
		panic(err)
	}
}

// Check runs the named gate against the principal.
func Check(name string, p *Principal) bool {
	gateMu.RLock()
	fn, ok := gates[name]
	gateMu.RUnlock()
	if !ok || fn == nil {
		return false
	}
	return fn(p)
}

// GateNames returns registered gate names (unordered).
func GateNames() []string {
	gateMu.RLock()
	defer gateMu.RUnlock()
	out := make([]string, 0, len(gates))
	for n := range gates {
		out = append(out, n)
	}
	return out
}

// ManagerGate returns a test helper that resolves credentials from r and authenticates via mgr.
func ManagerGate(mgr *Manager, guardName string, resolve CredentialResolver) func(*http.Request) (*Principal, error) {
	return func(r *http.Request) (*Principal, error) {
		cred, err := resolve(r)
		if err != nil {
			return nil, err
		}
		if cred == nil {
			return nil, adriver.ErrInvalidCredential
		}
		if guardName != "" {
			cred.Guard = guardName
		}
		return mgr.Authenticate(r.Context(), cred)
	}
}
