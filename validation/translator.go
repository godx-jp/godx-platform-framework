package validation

import (
	"fmt"
	"strings"
	"sync"
)

// Translator turns a FieldError into a human message. Implementations
// must be safe for concurrent use.
type Translator interface {
	Translate(fe FieldError) string
}

// DefaultTranslator returns an English translator with template
// strings for every built-in rule. Templates may use {field},
// {tag}, {param}, and {value} placeholders.
func DefaultTranslator() *MapTranslator {
	t := NewMapTranslator()
	for rule, template := range defaultEnglishTemplates {
		t.Add(rule, template)
	}
	t.SetFallback("{tag} failed {rule} validation")
	return t
}

// MapTranslator is the most common Translator — a map of
// rule-name to template string with placeholder substitution.
type MapTranslator struct {
	mu        sync.RWMutex
	templates map[string]string
	fallback  string
}

// NewMapTranslator returns an empty MapTranslator.
func NewMapTranslator() *MapTranslator {
	return &MapTranslator{templates: map[string]string{}}
}

// Add registers a template for a rule. Re-registering overwrites
// the previous template.
func (t *MapTranslator) Add(rule, template string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.templates[rule] = template
}

// SetFallback installs a template used for any rule that has no
// dedicated entry. The empty fallback yields FieldError.Error()
// style output.
func (t *MapTranslator) SetFallback(template string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.fallback = template
}

// Translate fills placeholders in the rule's template.
func (t *MapTranslator) Translate(fe FieldError) string {
	t.mu.RLock()
	template, ok := t.templates[fe.Rule]
	if !ok {
		template = t.fallback
	}
	t.mu.RUnlock()
	if template == "" {
		return ""
	}
	repl := strings.NewReplacer(
		"{field}", fe.Field,
		"{tag}", fe.Tag,
		"{rule}", fe.Rule,
		"{param}", fe.Param,
		"{value}", fmt.Sprint(fe.Value),
	)
	return repl.Replace(template)
}

// defaultEnglishTemplates is the canonical English message map.
// Templates use {tag} (preferred display name), {param}, {field}
// for the dotted path, and {value}. Add to this map when shipping
// new rules.
var defaultEnglishTemplates = map[string]string{
	"required":   "{tag} is required",
	"min":        "{tag} must be at least {param}",
	"max":        "{tag} must be at most {param}",
	"len":        "{tag} must be exactly {param} long",
	"between":    "{tag} must be between {param}",
	"eq":         "{tag} must equal {param}",
	"ne":         "{tag} must not equal {param}",
	"gt":         "{tag} must be greater than {param}",
	"gte":        "{tag} must be greater than or equal to {param}",
	"lt":         "{tag} must be less than {param}",
	"lte":        "{tag} must be less than or equal to {param}",
	"in":         "{tag} must be one of {param}",
	"oneof":      "{tag} must be one of {param}",
	"email":      "{tag} must be a valid email address",
	"url":        "{tag} must be a valid URL",
	"uuid":       "{tag} must be a valid UUID",
	"regex":      "{tag} must match the required pattern",
	"ip":         "{tag} must be a valid IP address",
	"ipv4":       "{tag} must be a valid IPv4 address",
	"ipv6":       "{tag} must be a valid IPv6 address",
	"alpha":      "{tag} must contain only letters",
	"numeric":    "{tag} must be numeric",
	"alphanum":   "{tag} must contain only letters and digits",
	"json":       "{tag} must be valid JSON",
	"startswith": "{tag} must start with {param}",
	"endswith":   "{tag} must end with {param}",
	"contains":   "{tag} must contain {param}",
	"eqfield":    "{tag} must equal {param}",
	"nefield":    "{tag} must not equal {param}",
	"gtfield":    "{tag} must be greater than {param}",
	"ltfield":    "{tag} must be less than {param}",
}
