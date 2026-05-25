package events

import "strings"

// match reports whether eventName matches pattern. Wildcard rules:
//
//	"*"            matches every event
//	"user.*"       matches every event whose name starts with "user."
//	"*.deleted"    matches every event whose name ends with ".deleted"
//	"user.created" exact match only
//	"user.*.email" matches "user.<anything>.email" (one segment)
//
// "*" inside a name matches one dot-separated segment. Multi-segment
// matching is achieved with a trailing or leading "*".
func match(pattern, eventName string) bool {
	if pattern == eventName {
		return true
	}
	if pattern == "*" {
		return true
	}
	if !strings.ContainsRune(pattern, '*') {
		return false
	}
	pp := strings.Split(pattern, ".")
	ep := strings.Split(eventName, ".")
	switch {
	case len(pp) > 0 && pp[0] == "*" && len(pp) == 1:
		return true
	case len(pp) >= 2 && pp[len(pp)-1] == "*":
		head := pp[:len(pp)-1]
		if len(ep) < len(head) {
			return false
		}
		for i, seg := range head {
			if seg == "*" {
				continue
			}
			if seg != ep[i] {
				return false
			}
		}
		return true
	case len(pp) >= 2 && pp[0] == "*":
		tail := pp[1:]
		if len(ep) < len(tail) {
			return false
		}
		offset := len(ep) - len(tail)
		for i, seg := range tail {
			if seg == "*" {
				continue
			}
			if seg != ep[offset+i] {
				return false
			}
		}
		return true
	}
	if len(pp) != len(ep) {
		return false
	}
	for i, seg := range pp {
		if seg == "*" {
			continue
		}
		if seg != ep[i] {
			return false
		}
	}
	return true
}
