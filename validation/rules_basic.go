package validation

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

var errFailed = errors.New("rule failed")

// numericLen returns (length, ok). For strings / slices / maps /
// arrays it returns the element count; for numeric kinds it returns
// the magnitude.
func numericLen(v any, k reflect.Kind) (float64, bool) {
	if v == nil {
		return 0, false
	}
	rv := reflect.ValueOf(v)
	switch k {
	case reflect.String:
		return float64(rv.Len()), true
	case reflect.Slice, reflect.Array, reflect.Map:
		return float64(rv.Len()), true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(rv.Int()), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(rv.Uint()), true
	case reflect.Float32, reflect.Float64:
		return rv.Float(), true
	}
	return 0, false
}

func parseFloat(p string) (float64, error) {
	v, err := strconv.ParseFloat(p, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %q", ErrInvalidParam, p)
	}
	return v, nil
}

// ruleRequired fails when the value is the zero value for its type.
func ruleRequired(rc RuleContext) error {
	if rc.Value == nil {
		return errFailed
	}
	rv := reflect.ValueOf(rc.Value)
	if !rv.IsValid() {
		return errFailed
	}
	if rv.Kind() == reflect.Ptr || rv.Kind() == reflect.Interface {
		if rv.IsNil() {
			return errFailed
		}
		rv = rv.Elem()
	}
	if !rv.IsValid() {
		return errFailed
	}
	if rv.IsZero() {
		return errFailed
	}
	return nil
}

// ruleMin: numeric values must be >= param; collections must have
// length >= param.
func ruleMin(rc RuleContext) error {
	threshold, err := parseFloat(rc.Param)
	if err != nil {
		return err
	}
	val, ok := numericLen(rc.Value, rc.Kind)
	if !ok {
		return errFailed
	}
	if val < threshold {
		return errFailed
	}
	return nil
}

// ruleMax: numeric or collection length <= param.
func ruleMax(rc RuleContext) error {
	threshold, err := parseFloat(rc.Param)
	if err != nil {
		return err
	}
	val, ok := numericLen(rc.Value, rc.Kind)
	if !ok {
		return errFailed
	}
	if val > threshold {
		return errFailed
	}
	return nil
}

// ruleLen: exact length / magnitude == param.
func ruleLen(rc RuleContext) error {
	want, err := parseFloat(rc.Param)
	if err != nil {
		return err
	}
	val, ok := numericLen(rc.Value, rc.Kind)
	if !ok {
		return errFailed
	}
	if val != want {
		return errFailed
	}
	return nil
}

// ruleBetween: param is "low|high" — magnitude / length within
// inclusive range.
func ruleBetween(rc RuleContext) error {
	parts := strings.Split(rc.Param, "|")
	if len(parts) != 2 {
		return fmt.Errorf("%w: %q (want low|high)", ErrInvalidParam, rc.Param)
	}
	lo, err := parseFloat(parts[0])
	if err != nil {
		return err
	}
	hi, err := parseFloat(parts[1])
	if err != nil {
		return err
	}
	val, ok := numericLen(rc.Value, rc.Kind)
	if !ok {
		return errFailed
	}
	if val < lo || val > hi {
		return errFailed
	}
	return nil
}

// stringValue returns the value as a string when it is one, else
// (empty, false).
func stringValue(v any, k reflect.Kind) (string, bool) {
	if k != reflect.String {
		return "", false
	}
	return reflect.ValueOf(v).String(), true
}
