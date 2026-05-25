package validation

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

// runRule is a tiny helper that wraps a rule call against a single
// value with optional param.
func runRule(t *testing.T, name string, value any, param string) error {
	t.Helper()
	v := New()
	rv := reflect.ValueOf(value)
	return v.rules[name](RuleContext{
		Ctx:   context.Background(),
		Field: "x",
		Tag:   "x",
		Value: value,
		Kind:  rv.Kind(),
		Param: param,
	})
}

func mustPass(t *testing.T, name string, value any, param string) {
	t.Helper()
	if err := runRule(t, name, value, param); err != nil {
		t.Fatalf("%s(%v,%q) expected pass, got %v", name, value, param, err)
	}
}

func mustFail(t *testing.T, name string, value any, param string) {
	t.Helper()
	if err := runRule(t, name, value, param); err == nil {
		t.Fatalf("%s(%v,%q) expected fail, got pass", name, value, param)
	}
}

func TestRuleRequired(t *testing.T) {
	mustPass(t, "required", "x", "")
	mustPass(t, "required", 1, "")
	mustPass(t, "required", []int{1}, "")
	mustFail(t, "required", "", "")
	mustFail(t, "required", 0, "")
	mustFail(t, "required", []int(nil), "")
	mustFail(t, "required", nil, "")
}

func TestRuleMinMaxLen(t *testing.T) {
	mustPass(t, "min", "abcd", "3")
	mustFail(t, "min", "ab", "3")
	mustPass(t, "max", "abcd", "5")
	mustFail(t, "max", "abcdef", "5")
	mustPass(t, "len", "abc", "3")
	mustFail(t, "len", "ab", "3")

	mustPass(t, "min", 10, "5")
	mustFail(t, "min", 4, "5")
	mustPass(t, "max", 5, "5")
	mustFail(t, "max", 6, "5")

	mustPass(t, "min", []string{"a", "b"}, "2")
	mustFail(t, "min", []string{"a"}, "2")
}

func TestRuleBetween(t *testing.T) {
	mustPass(t, "between", 5, "1|10")
	mustFail(t, "between", 11, "1|10")
	mustFail(t, "between", 0, "1|10")
	if err := runRule(t, "between", 5, "bad"); err == nil {
		t.Fatalf("expected invalid param")
	}
}

func TestRuleEqNe(t *testing.T) {
	mustPass(t, "eq", 5, "5")
	mustFail(t, "eq", 6, "5")
	mustPass(t, "ne", 6, "5")
	mustFail(t, "ne", 5, "5")
	mustPass(t, "eq", "abc", "abc")
	mustFail(t, "eq", "abc", "xyz")
}

func TestRuleComparisons(t *testing.T) {
	mustPass(t, "gt", 5, "3")
	mustFail(t, "gt", 3, "5")
	mustPass(t, "gte", 5, "5")
	mustFail(t, "gte", 4, "5")
	mustPass(t, "lt", 3, "5")
	mustFail(t, "lt", 5, "3")
	mustPass(t, "lte", 5, "5")
	mustFail(t, "lte", 6, "5")
}

func TestRuleIn(t *testing.T) {
	mustPass(t, "in", "active", "active|inactive|pending")
	mustFail(t, "in", "draft", "active|inactive|pending")
	mustPass(t, "in", 1, "1|2|3")
	mustFail(t, "in", 4, "1|2|3")
	// space-separated also works.
	mustPass(t, "in", "blue", "red green blue")
}

func TestRuleEmail(t *testing.T) {
	mustPass(t, "email", "a@b.com", "")
	mustFail(t, "email", "no-at-sign", "")
	mustFail(t, "email", "", "")
	mustFail(t, "email", "a@b.com <name>", "")
}

func TestRuleURL(t *testing.T) {
	mustPass(t, "url", "https://example.com", "")
	mustPass(t, "url", "http://x.y/path?q=1", "")
	mustFail(t, "url", "not-a-url", "")
	mustFail(t, "url", "", "")
}

func TestRuleUUID(t *testing.T) {
	mustPass(t, "uuid", "550e8400-e29b-41d4-a716-446655440000", "")
	mustFail(t, "uuid", "not-a-uuid", "")
}

func TestRuleRegex(t *testing.T) {
	mustPass(t, "regex", "abc123", "^[a-z0-9]+$")
	mustFail(t, "regex", "ABC", "^[a-z]+$")
	if err := runRule(t, "regex", "x", "[unclosed"); err == nil {
		t.Fatalf("expected compile err")
	}
}

func TestRuleIP(t *testing.T) {
	mustPass(t, "ip", "192.168.1.1", "")
	mustPass(t, "ip", "::1", "")
	mustFail(t, "ip", "999.999.999.999", "")
}

func TestRuleIPv4(t *testing.T) {
	mustPass(t, "ipv4", "1.2.3.4", "")
	mustFail(t, "ipv4", "::1", "")
}

func TestRuleIPv6(t *testing.T) {
	mustPass(t, "ipv6", "::1", "")
	mustFail(t, "ipv6", "1.2.3.4", "")
}

func TestRuleAlphaNumericAlphanum(t *testing.T) {
	mustPass(t, "alpha", "abcXYZ", "")
	mustFail(t, "alpha", "abc1", "")
	mustPass(t, "numeric", "12345", "")
	mustFail(t, "numeric", "12a", "")
	mustPass(t, "alphanum", "abc123XYZ", "")
	mustFail(t, "alphanum", "abc 123", "")
}

func TestRuleJSON(t *testing.T) {
	mustPass(t, "json", `{"a":1}`, "")
	mustPass(t, "json", `[1,2,3]`, "")
	mustFail(t, "json", `{a:1}`, "")
}

func TestRuleStringSubstrings(t *testing.T) {
	mustPass(t, "startswith", "hello world", "hello")
	mustFail(t, "startswith", "hello", "world")
	mustPass(t, "endswith", "hello.com", ".com")
	mustFail(t, "endswith", "hello", ".com")
	mustPass(t, "contains", "abcdef", "cd")
	mustFail(t, "contains", "abcdef", "xy")
}

func TestRuleCrossField(t *testing.T) {
	type form struct {
		Password string
		Confirm  string `validate:"eqfield=Password"`
	}
	v := New()
	if err := v.ValidateStruct(context.Background(), form{Password: "a", Confirm: "a"}); err != nil {
		t.Fatalf("eqfield ok: %v", err)
	}
	if err := v.ValidateStruct(context.Background(), form{Password: "a", Confirm: "b"}); err == nil {
		t.Fatalf("eqfield should fail")
	}

	type bounds struct {
		Min int
		Max int `validate:"gtfield=Min"`
	}
	if err := v.ValidateStruct(context.Background(), bounds{Min: 1, Max: 5}); err != nil {
		t.Fatalf("gtfield ok: %v", err)
	}
	if err := v.ValidateStruct(context.Background(), bounds{Min: 5, Max: 1}); err == nil {
		t.Fatalf("gtfield should fail")
	}
}

func TestRuleCrossFieldMissingSibling(t *testing.T) {
	type bad struct {
		X int `validate:"eqfield=NoSuch"`
	}
	v := New()
	err := v.ValidateStruct(context.Background(), bad{X: 5})
	if err == nil {
		t.Fatalf("expected err")
	}
	if !strings.Contains(err.Error(), "NoSuch") {
		t.Fatalf("err should mention sibling: %v", err)
	}
}

func TestNumericLenSupportsAllKinds(t *testing.T) {
	cases := []struct {
		v    any
		want float64
		ok   bool
	}{
		{"abc", 3, true},
		{[]int{1, 2}, 2, true},
		{map[string]int{"a": 1}, 1, true},
		{int8(7), 7, true},
		{uint16(3), 3, true},
		{1.5, 1.5, true},
		{nil, 0, false},
		{struct{}{}, 0, false},
	}
	for i, tc := range cases {
		rv := reflect.ValueOf(tc.v)
		got, ok := numericLen(tc.v, rv.Kind())
		if ok != tc.ok || got != tc.want {
			t.Fatalf("[%d] %v: got=(%v,%v) want=(%v,%v)", i, tc.v, got, ok, tc.want, tc.ok)
		}
	}
}
