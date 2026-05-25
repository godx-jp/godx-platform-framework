package validation

import (
	"fmt"
	"strings"
)

// ruleCall is one rule invocation parsed out of a struct tag.
type ruleCall struct {
	name  string
	param string
}

// parseTag splits a `validate:"..."` tag value into individual rule
// calls. Each call is `name` or `name=param`. Calls are separated
// by commas; parameters may quote with single quotes to escape
// commas and equals signs inside the value.
//
//	"required,email,max=255"   -> [{required ""} {email ""} {max "255"}]
//	"oneof='a,b,c'"            -> [{oneof "a,b,c"}]
//
// Returns ErrInvalidTag when the tag is malformed.
func parseTag(tag string) ([]ruleCall, error) {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return nil, nil
	}
	var (
		out      []ruleCall
		buf      strings.Builder
		inQuotes bool
	)
	flush := func(strict bool) error {
		raw := strings.TrimSpace(buf.String())
		buf.Reset()
		if raw == "" {
			if strict {
				return fmt.Errorf("%w: empty rule in %q", ErrInvalidTag, tag)
			}
			return nil
		}
		eq := indexOutsideQuotes(raw, '=')
		var rc ruleCall
		if eq < 0 {
			rc.name = strings.TrimSpace(raw)
		} else {
			rc.name = strings.TrimSpace(raw[:eq])
			rc.param = unquote(strings.TrimSpace(raw[eq+1:]))
		}
		if rc.name == "" {
			return fmt.Errorf("%w: empty rule name in %q", ErrInvalidTag, tag)
		}
		out = append(out, rc)
		return nil
	}
	for i := 0; i < len(tag); i++ {
		c := tag[i]
		switch c {
		case '\'':
			inQuotes = !inQuotes
			buf.WriteByte(c)
		case ',':
			if inQuotes {
				buf.WriteByte(c)
				continue
			}
			if err := flush(true); err != nil {
				return nil, err
			}
		default:
			buf.WriteByte(c)
		}
	}
	if inQuotes {
		return nil, fmt.Errorf("%w: unterminated quote in %q", ErrInvalidTag, tag)
	}
	if err := flush(false); err != nil {
		return nil, err
	}
	return out, nil
}

func indexOutsideQuotes(s string, target byte) int {
	inQuotes := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\'' {
			inQuotes = !inQuotes
			continue
		}
		if !inQuotes && c == target {
			return i
		}
	}
	return -1
}

func unquote(s string) string {
	if len(s) >= 2 && s[0] == '\'' && s[len(s)-1] == '\'' {
		return s[1 : len(s)-1]
	}
	return s
}

// jsonTagName extracts the field name from a json struct tag. Empty
// or "-" returns the empty string.
func jsonTagName(tag string) string {
	if tag == "" || tag == "-" {
		return ""
	}
	if i := strings.IndexByte(tag, ','); i >= 0 {
		return tag[:i]
	}
	return tag
}
