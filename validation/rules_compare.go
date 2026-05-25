package validation

import (
	"fmt"
	"reflect"
)

// compareNumeric returns (-1, 0, +1, ok). ok=false when types are
// incomparable.
func compareNumeric(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

// numericValue extracts a comparable float64 from rc.Value when
// possible.
func numericValue(v any, k reflect.Kind) (float64, bool) {
	return numericLen(v, k)
}

// ruleEq: value == param. Strings compare literally; numeric kinds
// compare numerically. Length-based eq for collections is handled
// by `len` instead.
func ruleEq(rc RuleContext) error {
	if s, ok := stringValue(rc.Value, rc.Kind); ok {
		if s == rc.Param {
			return nil
		}
		return errFailed
	}
	if v, ok := numericValue(rc.Value, rc.Kind); ok && rc.Kind != reflect.Slice && rc.Kind != reflect.Map && rc.Kind != reflect.Array {
		t, err := parseFloat(rc.Param)
		if err != nil {
			return err
		}
		if compareNumeric(v, t) == 0 {
			return nil
		}
		return errFailed
	}
	return errFailed
}

// ruleNe: value != param.
func ruleNe(rc RuleContext) error {
	if err := ruleEq(rc); err == nil {
		return errFailed
	} else if err == errFailed {
		return nil
	} else {
		return err
	}
}

func ruleGt(rc RuleContext) error  { return cmpRule(rc, func(c int) bool { return c > 0 }) }
func ruleGte(rc RuleContext) error { return cmpRule(rc, func(c int) bool { return c >= 0 }) }
func ruleLt(rc RuleContext) error  { return cmpRule(rc, func(c int) bool { return c < 0 }) }
func ruleLte(rc RuleContext) error { return cmpRule(rc, func(c int) bool { return c <= 0 }) }

func cmpRule(rc RuleContext, want func(int) bool) error {
	v, ok := numericValue(rc.Value, rc.Kind)
	if !ok {
		return fmt.Errorf("%w: %s requires numeric value", ErrInvalidParam, rc.Param)
	}
	t, err := parseFloat(rc.Param)
	if err != nil {
		return err
	}
	if want(compareNumeric(v, t)) {
		return nil
	}
	return errFailed
}

// crossField finds a sibling field by name in rc.Parent. Returns
// (value, ok). The lookup is case-sensitive against Go field names.
func crossField(rc RuleContext) (reflect.Value, bool) {
	if !rc.Parent.IsValid() || rc.Parent.Kind() != reflect.Struct {
		return reflect.Value{}, false
	}
	fv := rc.Parent.FieldByName(rc.Param)
	if !fv.IsValid() {
		return reflect.Value{}, false
	}
	return fv, true
}

// ruleEqField: this field equals the named sibling.
func ruleEqField(rc RuleContext) error {
	other, ok := crossField(rc)
	if !ok {
		return fmt.Errorf("%w: sibling %q not found", ErrInvalidParam, rc.Param)
	}
	if reflect.DeepEqual(rc.Value, other.Interface()) {
		return nil
	}
	return errFailed
}

// ruleNeField: this field differs from the named sibling.
func ruleNeField(rc RuleContext) error {
	other, ok := crossField(rc)
	if !ok {
		return fmt.Errorf("%w: sibling %q not found", ErrInvalidParam, rc.Param)
	}
	if !reflect.DeepEqual(rc.Value, other.Interface()) {
		return nil
	}
	return errFailed
}

// ruleGtField / ruleLtField compare magnitudes when both fields are
// numeric / lengths.
func ruleGtField(rc RuleContext) error { return cmpFieldRule(rc, func(c int) bool { return c > 0 }) }
func ruleLtField(rc RuleContext) error { return cmpFieldRule(rc, func(c int) bool { return c < 0 }) }

func cmpFieldRule(rc RuleContext, want func(int) bool) error {
	other, ok := crossField(rc)
	if !ok {
		return fmt.Errorf("%w: sibling %q not found", ErrInvalidParam, rc.Param)
	}
	v, ok1 := numericValue(rc.Value, rc.Kind)
	o, ok2 := numericValue(other.Interface(), other.Kind())
	if !ok1 || !ok2 {
		return fmt.Errorf("%w: cross-field compare needs numeric kinds", ErrInvalidParam)
	}
	if want(compareNumeric(v, o)) {
		return nil
	}
	return errFailed
}
