package validation

import (
	"fmt"
	"strings"
)

// ruleIn / ruleOneOf accept a space-or-pipe-separated whitelist.
// Numeric / string values both work; comparison is by string form.
func ruleIn(rc RuleContext) error {
	if rc.Param == "" {
		return fmt.Errorf("%w: in rule needs values", ErrInvalidParam)
	}
	want := splitSet(rc.Param)
	current := stringForm(rc.Value)
	for _, w := range want {
		if w == current {
			return nil
		}
	}
	return errFailed
}

func ruleOneOf(rc RuleContext) error { return ruleIn(rc) }

func splitSet(p string) []string {
	if strings.ContainsRune(p, '|') {
		return splitTrim(p, "|")
	}
	return splitTrim(p, " ")
}

func splitTrim(s, sep string) []string {
	parts := strings.Split(s, sep)
	out := make([]string, 0, len(parts))
	for _, x := range parts {
		x = strings.TrimSpace(x)
		if x == "" {
			continue
		}
		out = append(out, x)
	}
	return out
}

func stringForm(v any) string {
	return fmt.Sprint(v)
}
