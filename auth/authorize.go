package auth

import "sync"

// BeforeHook runs before gate/policy evaluation. Return nil to defer; non-nil short-circuits.
type BeforeHook func(p *Principal, ability string, args ...any) *bool

// AfterHook runs after gate/policy. Return pointer false to deny; true or nil defer to allow.
type AfterHook func(p *Principal, ability string, args ...any) *bool

var (
	hookMu      sync.RWMutex
	beforeHooks []BeforeHook
	afterHooks  []AfterHook
)

// Before registers a global before hook (Laravel Gate::before).
func Before(h BeforeHook) {
	if h == nil {
		return
	}
	hookMu.Lock()
	beforeHooks = append(beforeHooks, h)
	hookMu.Unlock()
}

// After registers a global after hook (Laravel Gate::after).
func After(h AfterHook) {
	if h == nil {
		return
	}
	hookMu.Lock()
	afterHooks = append(afterHooks, h)
	hookMu.Unlock()
}

// ResetHooks clears all before/after hooks (for tests).
func ResetHooks() {
	hookMu.Lock()
	beforeHooks = nil
	afterHooks = nil
	hookMu.Unlock()
}

// Authorize evaluates ability for principal and optional resource args.
func Authorize(ability string, p *Principal, args ...any) bool {
	if ability == "" {
		return false
	}
	if r := runBeforeHooks(p, ability, args...); r != nil {
		return *r
	}
	allowed := false
	matched := false
	if len(args) == 0 || args[0] == nil {
		if fn, ok := lookupGate(ability); ok {
			matched = true
			allowed = fn(p)
		}
	} else {
		if pol, ok := policyFor(args[0]); ok {
			matched = true
			allowed = pol.Authorize(p, ability, args[0])
		}
		if !matched {
			if fn, ok := lookupGate(ability); ok {
				matched = true
				allowed = fn(p)
			}
		}
	}
	if !matched {
		return false
	}
	if !allowed {
		return false
	}
	if r := runAfterHooks(p, ability, args...); r != nil {
		return *r
	}
	return true
}

// Check runs Authorize (backward compatible with optional resource args).
func Check(name string, p *Principal, args ...any) bool {
	return Authorize(name, p, args...)
}

func runBeforeHooks(p *Principal, ability string, args ...any) *bool {
	hookMu.RLock()
	hooks := append([]BeforeHook(nil), beforeHooks...)
	hookMu.RUnlock()
	for _, h := range hooks {
		if r := h(p, ability, args...); r != nil {
			return r
		}
	}
	return nil
}

func runAfterHooks(p *Principal, ability string, args ...any) *bool {
	hookMu.RLock()
	hooks := append([]AfterHook(nil), afterHooks...)
	hookMu.RUnlock()
	for _, h := range hooks {
		if r := h(p, ability, args...); r != nil {
			return r
		}
	}
	return nil
}

func lookupGate(name string) (GateFunc, bool) {
	gateMu.RLock()
	fn, ok := gates[name]
	gateMu.RUnlock()
	return fn, ok && fn != nil
}
