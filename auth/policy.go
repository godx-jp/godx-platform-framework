package auth

import (
	"fmt"
	"reflect"
	"sync"
)

// Policy authorizes abilities against a typed resource.
type Policy interface {
	Authorize(p *Principal, ability string, resource any) bool
}

var (
	policyMu sync.RWMutex
	policies = map[reflect.Type]Policy{}
	funcPol  = map[reflect.Type]map[string]any{}
)

// RegisterPolicy binds a Policy to resource type T.
func RegisterPolicy[T any](p Policy) error {
	if p == nil {
		return fmt.Errorf("auth: RegisterPolicy: nil policy")
	}
	var zero T
	t := reflectTypeOf(zero)
	policyMu.Lock()
	defer policyMu.Unlock()
	if _, exists := policies[t]; exists {
		return fmt.Errorf("auth: policy for %v already registered", t)
	}
	policies[t] = p
	return nil
}

// RegisterPolicyFunc registers a single-ability handler for resource type T.
func RegisterPolicyFunc[T any](ability string, fn func(*Principal, T) bool) error {
	if ability == "" || fn == nil {
		return fmt.Errorf("auth: RegisterPolicyFunc: ability and fn required")
	}
	var zero T
	t := reflectTypeOf(zero)
	policyMu.Lock()
	defer policyMu.Unlock()
	m, ok := funcPol[t]
	if !ok {
		m = map[string]any{}
		funcPol[t] = m
	}
	if _, exists := m[ability]; exists {
		return fmt.Errorf("auth: policy func %q for %v already registered", ability, t)
	}
	m[ability] = fn
	return nil
}

// MustRegisterPolicy registers a policy and panics on error.
func MustRegisterPolicy[T any](p Policy) {
	if err := RegisterPolicy[T](p); err != nil {
		panic(err)
	}
}

// ResetPolicies clears registered policies (for tests).
func ResetPolicies() {
	policyMu.Lock()
	policies = map[reflect.Type]Policy{}
	funcPol = map[reflect.Type]map[string]any{}
	policyMu.Unlock()
}

type funcPolicy struct {
	t       reflect.Type
	ability map[string]any
}

func (fp funcPolicy) Authorize(p *Principal, ability string, resource any) bool {
	fnAny, ok := fp.ability[ability]
	if !ok {
		return false
	}
	rv := reflect.ValueOf(resource)
	if !rv.IsValid() {
		return false
	}
	rt := rv.Type()
	if rt != fp.t {
		if rt.Kind() == reflect.Pointer && rt.Elem() == fp.t {
			rv = rv.Elem()
		} else {
			return false
		}
	}
	results := reflect.ValueOf(fnAny).Call([]reflect.Value{
		reflect.ValueOf(p),
		rv,
	})
	if len(results) != 1 {
		return false
	}
	return results[0].Bool()
}

func policyFor(resource any) (Policy, bool) {
	if resource == nil {
		return nil, false
	}
	t := reflectTypeOf(resource)
	policyMu.RLock()
	defer policyMu.RUnlock()
	if p, ok := policies[t]; ok {
		return p, true
	}
	if m, ok := funcPol[t]; ok && len(m) > 0 {
		return funcPolicy{t: t, ability: m}, true
	}
	if rt := reflect.TypeOf(resource); rt.Kind() == reflect.Pointer {
		if p, ok := policies[rt.Elem()]; ok {
			return p, true
		}
		if m, ok := funcPol[rt.Elem()]; ok && len(m) > 0 {
			return funcPolicy{t: rt.Elem(), ability: m}, true
		}
	}
	return nil, false
}

func reflectTypeOf(v any) reflect.Type {
	t := reflect.TypeOf(v)
	if t == nil {
		return nil
	}
	if t.Kind() == reflect.Pointer {
		return t.Elem()
	}
	return t
}
