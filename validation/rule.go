package validation

import (
	"context"
	"reflect"
)

// RuleContext gives a Rule everything it needs to make a decision
// without forcing callers to walk the reflection graph themselves.
type RuleContext struct {
	// Ctx is the caller's context — propagate cancellation /
	// deadlines through Rules that perform IO (none today; built
	// for future extensibility).
	Ctx context.Context

	// Field is the dotted path to the field under validation.
	Field string

	// Tag is the human-readable field tag (JSON tag when present,
	// else the Go field name).
	Tag string

	// Value is the field's current value.
	Value any

	// Kind is reflect.Kind of Value. Pre-computed so rules don't
	// have to use reflection for the common case.
	Kind reflect.Kind

	// Param is the rule parameter as it appears in the struct tag
	// (e.g. "8" for `min=8`). Empty for parameter-less rules.
	Param string

	// Parent gives cross-field rules access to siblings. Nil when
	// the field has no parent struct (e.g. a top-level scalar passed
	// directly to Validate).
	Parent reflect.Value
}

// Rule is the contract for a single validation check. Return nil
// to pass; return an error to fail. The returned error's message
// is ignored — Validator builds the FieldError from the rule name
// and runs it through the Translator instead. To return a
// machine-readable rule-internal error (e.g. unparseable
// parameter), wrap ErrInvalidParam.
type Rule func(rc RuleContext) error

// ruleEntry is the internal record of one registered rule.
type ruleEntry struct {
	name string
	rule Rule
}
