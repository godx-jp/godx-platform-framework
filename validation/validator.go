package validation

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"sync"
)

// Validator orchestrates rule execution against structs.
type Validator struct {
	mu         sync.RWMutex
	rules      map[string]Rule
	translator Translator

	cacheMu sync.RWMutex
	cache   map[reflect.Type]*typeRules
}

// New returns a Validator populated with built-in rules and the
// default English translator.
func New() *Validator {
	v := &Validator{
		rules:      map[string]Rule{},
		translator: DefaultTranslator(),
		cache:      map[reflect.Type]*typeRules{},
	}
	v.registerBuiltins()
	return v
}

// AddRule registers a Rule under name. Re-registering overwrites
// the previous rule, allowing built-ins to be swapped out.
func (v *Validator) AddRule(name string, r Rule) error {
	if name == "" {
		return errors.New("validation: AddRule: name is required")
	}
	if r == nil {
		return fmt.Errorf("validation: AddRule(%q): rule is nil", name)
	}
	v.mu.Lock()
	v.rules[name] = r
	v.mu.Unlock()
	v.invalidateCache()
	return nil
}

// SetTranslator swaps the active Translator. Use NewMapTranslator
// to customise messages without writing a new implementation.
func (v *Validator) SetTranslator(t Translator) {
	if t == nil {
		return
	}
	v.mu.Lock()
	v.translator = t
	v.mu.Unlock()
}

// Translator returns the active Translator.
func (v *Validator) Translator() Translator {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.translator
}

// Rules returns the sorted list of registered rule names.
func (v *Validator) Rules() []string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	out := make([]string, 0, len(v.rules))
	for n := range v.rules {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// HasRule reports whether name is registered.
func (v *Validator) HasRule(name string) bool {
	v.mu.RLock()
	_, ok := v.rules[name]
	v.mu.RUnlock()
	return ok
}

// ValidateStruct validates v against its struct tags. Returns nil
// when no rules fail. Returns Errors when one or more rules fail.
// Returns ErrNotStruct (wrapped) when value is not a struct or
// pointer to struct. Returns a wrapped ErrInvalidTag /
// ErrUnknownRule when the struct's tags themselves are malformed —
// these are programmer errors not validation failures, so they are
// distinct from Errors.
func (v *Validator) ValidateStruct(ctx context.Context, value any) error {
	rv := reflect.ValueOf(value)
	for rv.Kind() == reflect.Ptr || rv.Kind() == reflect.Interface {
		if rv.IsNil() {
			return ErrNotStruct
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return fmt.Errorf("%w: got %s", ErrNotStruct, rv.Kind())
	}
	rules, err := v.compileType(rv.Type())
	if err != nil {
		return err
	}
	var errs Errors
	v.validateStructValue(ctx, rv, "", rules, &errs)
	return errs.AsError()
}

// ValidateField validates a single value against the given tag
// expression — useful for ad-hoc / dynamic checks outside structs.
func (v *Validator) ValidateField(ctx context.Context, value any, tag string) error {
	calls, err := parseTag(tag)
	if err != nil {
		return err
	}
	rv := reflect.ValueOf(value)
	rc := RuleContext{
		Ctx:   ctx,
		Field: "",
		Tag:   "",
		Value: value,
		Kind:  rv.Kind(),
	}
	var errs Errors
	for _, c := range calls {
		v.runOne(rc, c, &errs)
	}
	return errs.AsError()
}

// runOne resolves a rule name, executes it, and appends a FieldError
// to errs on failure. Internal rule errors (ErrInvalidParam, etc.)
// also produce a FieldError so users see them in context.
func (v *Validator) runOne(rc RuleContext, call ruleCall, errs *Errors) {
	v.mu.RLock()
	rule, ok := v.rules[call.name]
	tr := v.translator
	v.mu.RUnlock()
	if !ok {
		fe := FieldError{
			Field:   rc.Field,
			Tag:     rc.Tag,
			Rule:    call.name,
			Param:   call.param,
			Value:   rc.Value,
			Message: fmt.Sprintf("unknown rule %q", call.name),
		}
		errs.Add(fe)
		return
	}
	rc.Param = call.param
	if err := rule(rc); err != nil {
		fe := FieldError{
			Field: rc.Field,
			Tag:   rc.Tag,
			Rule:  call.name,
			Param: call.param,
			Value: rc.Value,
		}
		if tr != nil {
			fe.Message = tr.Translate(fe)
		}
		if fe.Message == "" {
			fe.Message = err.Error()
		}
		errs.Add(fe)
	}
}

// validateStructValue walks struct fields, runs configured rules,
// and recurses into nested struct fields. prefix is the dotted
// path to this struct (empty at top level).
func (v *Validator) validateStructValue(ctx context.Context, rv reflect.Value, prefix string, tr *typeRules, errs *Errors) {
	for _, fr := range tr.fields {
		fv := rv.Field(fr.index)
		field := fr.name
		if prefix != "" {
			field = prefix + "." + fr.name
		}
		val := fv.Interface()
		rc := RuleContext{
			Ctx:    ctx,
			Field:  field,
			Tag:    fr.tagName,
			Value:  val,
			Kind:   fv.Kind(),
			Parent: rv,
		}
		// Skip non-required rules when the value is the zero value
		// AND the field isn't tagged required — Laravel-style nullable.
		isZero := fv.IsZero()
		hasRequired := false
		for _, c := range fr.calls {
			if c.name == "required" {
				hasRequired = true
				break
			}
		}
		if isZero && !hasRequired {
			continue
		}
		for _, c := range fr.calls {
			v.runOne(rc, c, errs)
		}
		// Recurse into nested struct / pointer-to-struct fields.
		nested := fv
		if nested.Kind() == reflect.Ptr {
			if nested.IsNil() {
				continue
			}
			nested = nested.Elem()
		}
		if nested.Kind() == reflect.Struct {
			nestedRules, err := v.compileType(nested.Type())
			if err != nil {
				errs.Add(FieldError{Field: field, Message: err.Error()})
				continue
			}
			if len(nestedRules.fields) > 0 {
				v.validateStructValue(ctx, nested, field, nestedRules, errs)
			}
		}
	}
}

// fieldRules holds compiled tag info for one struct field.
type fieldRules struct {
	index   int
	name    string
	tagName string
	calls   []ruleCall
}

// typeRules is the per-type compiled rule cache.
type typeRules struct {
	fields []fieldRules
}

func (v *Validator) compileType(t reflect.Type) (*typeRules, error) {
	v.cacheMu.RLock()
	if tr, ok := v.cache[t]; ok {
		v.cacheMu.RUnlock()
		return tr, nil
	}
	v.cacheMu.RUnlock()

	tr := &typeRules{}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		tag := f.Tag.Get("validate")
		calls, err := parseTag(tag)
		if err != nil {
			return nil, err
		}
		display := jsonTagName(f.Tag.Get("json"))
		if display == "" {
			display = f.Name
		}
		// Skip fields without rules unless they're nested structs
		// (need recursion).
		if len(calls) == 0 {
			ft := f.Type
			for ft.Kind() == reflect.Ptr {
				ft = ft.Elem()
			}
			if ft.Kind() != reflect.Struct {
				continue
			}
		}
		// Validate rule names against the registry early so typos
		// surface at compile time, not silently.
		v.mu.RLock()
		for _, c := range calls {
			if _, ok := v.rules[c.name]; !ok {
				v.mu.RUnlock()
				return nil, fmt.Errorf("%w: %q (field %s.%s)", ErrUnknownRule, c.name, t.Name(), f.Name)
			}
		}
		v.mu.RUnlock()
		tr.fields = append(tr.fields, fieldRules{
			index:   i,
			name:    f.Name,
			tagName: display,
			calls:   calls,
		})
	}
	v.cacheMu.Lock()
	v.cache[t] = tr
	v.cacheMu.Unlock()
	return tr, nil
}

func (v *Validator) invalidateCache() {
	v.cacheMu.Lock()
	v.cache = map[reflect.Type]*typeRules{}
	v.cacheMu.Unlock()
}

func (v *Validator) registerBuiltins() {
	v.rules["required"] = ruleRequired
	v.rules["min"] = ruleMin
	v.rules["max"] = ruleMax
	v.rules["len"] = ruleLen
	v.rules["between"] = ruleBetween
	v.rules["eq"] = ruleEq
	v.rules["ne"] = ruleNe
	v.rules["gt"] = ruleGt
	v.rules["gte"] = ruleGte
	v.rules["lt"] = ruleLt
	v.rules["lte"] = ruleLte
	v.rules["in"] = ruleIn
	v.rules["oneof"] = ruleOneOf
	v.rules["email"] = ruleEmail
	v.rules["url"] = ruleURL
	v.rules["uuid"] = ruleUUID
	v.rules["regex"] = ruleRegex
	v.rules["ip"] = ruleIP
	v.rules["ipv4"] = ruleIPv4
	v.rules["ipv6"] = ruleIPv6
	v.rules["alpha"] = ruleAlpha
	v.rules["numeric"] = ruleNumeric
	v.rules["alphanum"] = ruleAlphanum
	v.rules["json"] = ruleJSON
	v.rules["startswith"] = ruleStartsWith
	v.rules["endswith"] = ruleEndsWith
	v.rules["contains"] = ruleContains
	v.rules["eqfield"] = ruleEqField
	v.rules["nefield"] = ruleNeField
	v.rules["gtfield"] = ruleGtField
	v.rules["ltfield"] = ruleLtField
}
