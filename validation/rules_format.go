package validation

import (
	"encoding/json"
	"fmt"
	"net"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"sync"
)

var (
	uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

	regexCacheMu sync.RWMutex
	regexCache   = map[string]*regexp.Regexp{}
)

func compileRegex(pat string) (*regexp.Regexp, error) {
	regexCacheMu.RLock()
	if re, ok := regexCache[pat]; ok {
		regexCacheMu.RUnlock()
		return re, nil
	}
	regexCacheMu.RUnlock()
	re, err := regexp.Compile(pat)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidParam, err)
	}
	regexCacheMu.Lock()
	regexCache[pat] = re
	regexCacheMu.Unlock()
	return re, nil
}

func ruleEmail(rc RuleContext) error {
	s, ok := stringValue(rc.Value, rc.Kind)
	if !ok || s == "" {
		return errFailed
	}
	addr, err := mail.ParseAddress(s)
	if err != nil || addr.Address != s {
		return errFailed
	}
	return nil
}

func ruleURL(rc RuleContext) error {
	s, ok := stringValue(rc.Value, rc.Kind)
	if !ok || s == "" {
		return errFailed
	}
	u, err := url.ParseRequestURI(s)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return errFailed
	}
	return nil
}

func ruleUUID(rc RuleContext) error {
	s, ok := stringValue(rc.Value, rc.Kind)
	if !ok || !uuidRe.MatchString(s) {
		return errFailed
	}
	return nil
}

func ruleRegex(rc RuleContext) error {
	s, ok := stringValue(rc.Value, rc.Kind)
	if !ok {
		return errFailed
	}
	if rc.Param == "" {
		return fmt.Errorf("%w: regex rule needs a pattern", ErrInvalidParam)
	}
	re, err := compileRegex(rc.Param)
	if err != nil {
		return err
	}
	if !re.MatchString(s) {
		return errFailed
	}
	return nil
}

func ruleIP(rc RuleContext) error {
	s, ok := stringValue(rc.Value, rc.Kind)
	if !ok || net.ParseIP(s) == nil {
		return errFailed
	}
	return nil
}

func ruleIPv4(rc RuleContext) error {
	s, ok := stringValue(rc.Value, rc.Kind)
	if !ok {
		return errFailed
	}
	ip := net.ParseIP(s)
	if ip == nil || ip.To4() == nil || strings.Contains(s, ":") {
		return errFailed
	}
	return nil
}

func ruleIPv6(rc RuleContext) error {
	s, ok := stringValue(rc.Value, rc.Kind)
	if !ok {
		return errFailed
	}
	ip := net.ParseIP(s)
	if ip == nil || !strings.Contains(s, ":") {
		return errFailed
	}
	return nil
}

func ruleAlpha(rc RuleContext) error {
	s, ok := stringValue(rc.Value, rc.Kind)
	if !ok || s == "" {
		return errFailed
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') {
			return errFailed
		}
	}
	return nil
}

func ruleNumeric(rc RuleContext) error {
	s, ok := stringValue(rc.Value, rc.Kind)
	if !ok || s == "" {
		return errFailed
	}
	for _, r := range s {
		if !(r >= '0' && r <= '9') {
			return errFailed
		}
	}
	return nil
}

func ruleAlphanum(rc RuleContext) error {
	s, ok := stringValue(rc.Value, rc.Kind)
	if !ok || s == "" {
		return errFailed
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		default:
			return errFailed
		}
	}
	return nil
}

func ruleJSON(rc RuleContext) error {
	s, ok := stringValue(rc.Value, rc.Kind)
	if !ok {
		return errFailed
	}
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return errFailed
	}
	return nil
}

func ruleStartsWith(rc RuleContext) error {
	s, ok := stringValue(rc.Value, rc.Kind)
	if !ok {
		return errFailed
	}
	if !strings.HasPrefix(s, rc.Param) {
		return errFailed
	}
	return nil
}

func ruleEndsWith(rc RuleContext) error {
	s, ok := stringValue(rc.Value, rc.Kind)
	if !ok {
		return errFailed
	}
	if !strings.HasSuffix(s, rc.Param) {
		return errFailed
	}
	return nil
}

func ruleContains(rc RuleContext) error {
	s, ok := stringValue(rc.Value, rc.Kind)
	if !ok {
		return errFailed
	}
	if !strings.Contains(s, rc.Param) {
		return errFailed
	}
	return nil
}
