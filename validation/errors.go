package validation

import (
	"errors"
	"fmt"
	"strings"
)

// FieldError captures one rule violation against one field. Field
// is the dotted JSON-style path (Customer.Address.ZIP). Tag is the
// (struct-tag) field name when a `json` tag is present; otherwise
// the Go field name. Rule is the registered rule name (e.g.
// "required", "min"). Param is the rule parameter as it appeared in
// the struct tag (e.g. "8" for `min=8`).
type FieldError struct {
	Field   string
	Tag     string
	Rule    string
	Param   string
	Value   any
	Message string
}

// Error returns a human-readable string. The Validator's Translator
// fills Message at construction time; this method falls back to a
// machine-readable form when Message is empty.
func (f FieldError) Error() string {
	if f.Message != "" {
		return f.Field + ": " + f.Message
	}
	if f.Param != "" {
		return fmt.Sprintf("%s: %s(%s) failed", f.Field, f.Rule, f.Param)
	}
	return fmt.Sprintf("%s: %s failed", f.Field, f.Rule)
}

// Errors aggregates per-field errors. The slice satisfies the error
// interface and unwraps to individual FieldError values via
// errors.As (use a *FieldError target for each entry).
type Errors []FieldError

// Error returns all per-field messages, separated by "; ". An empty
// slice returns the empty string but is never a non-nil error in
// practice — Validator returns nil when there are no failures.
func (e Errors) Error() string {
	if len(e) == 0 {
		return ""
	}
	parts := make([]string, 0, len(e))
	for _, fe := range e {
		parts = append(parts, fe.Error())
	}
	return strings.Join(parts, "; ")
}

// Has reports whether at least one field failed any rule.
func (e Errors) Has() bool { return len(e) > 0 }

// HasField reports whether field has at least one violation.
func (e Errors) HasField(field string) bool {
	for _, fe := range e {
		if fe.Field == field {
			return true
		}
	}
	return false
}

// FieldErrors returns every violation against field.
func (e Errors) FieldErrors(field string) []FieldError {
	out := make([]FieldError, 0)
	for _, fe := range e {
		if fe.Field == field {
			out = append(out, fe)
		}
	}
	return out
}

// AsError returns e cast to error, or nil if empty. Useful when
// composing validation calls so callers can naturally check
// `err != nil`.
func (e Errors) AsError() error {
	if len(e) == 0 {
		return nil
	}
	return e
}

// Add appends a single FieldError.
func (e *Errors) Add(fe FieldError) { *e = append(*e, fe) }

// Sentinel errors.
var (
	// ErrUnknownRule is returned when a struct tag references a rule
	// that is not registered.
	ErrUnknownRule = errors.New("validation: unknown rule")

	// ErrInvalidTag is returned when the validate tag is malformed
	// (e.g. unbalanced quotes, empty rule name).
	ErrInvalidTag = errors.New("validation: invalid tag")

	// ErrNotStruct is returned by ValidateStruct when the value
	// passed is not a struct (or pointer to struct).
	ErrNotStruct = errors.New("validation: value is not a struct")

	// ErrInvalidParam is returned by a rule when its parameter is
	// malformed (e.g. min=abc — abc is not numeric).
	ErrInvalidParam = errors.New("validation: invalid rule parameter")
)
