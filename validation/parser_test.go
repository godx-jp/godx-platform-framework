package validation

import (
	"errors"
	"testing"
)

func TestParseTagEmpty(t *testing.T) {
	calls, err := parseTag("")
	if err != nil || len(calls) != 0 {
		t.Fatalf("empty tag: calls=%v err=%v", calls, err)
	}
}

func TestParseTagSimple(t *testing.T) {
	calls, err := parseTag("required,email,max=255")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := []ruleCall{
		{name: "required"},
		{name: "email"},
		{name: "max", param: "255"},
	}
	if len(calls) != 3 {
		t.Fatalf("len=%d", len(calls))
	}
	for i, w := range want {
		if calls[i] != w {
			t.Fatalf("[%d] got %+v want %+v", i, calls[i], w)
		}
	}
}

func TestParseTagQuotedParam(t *testing.T) {
	calls, err := parseTag("oneof='a,b,c'")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(calls) != 1 || calls[0].name != "oneof" || calls[0].param != "a,b,c" {
		t.Fatalf("got %+v", calls)
	}
}

func TestParseTagInvalid(t *testing.T) {
	for _, in := range []string{
		",",
		"foo,,bar",
		"'unterminated",
		"=missingname",
	} {
		_, err := parseTag(in)
		if err == nil || !errors.Is(err, ErrInvalidTag) {
			t.Fatalf("expected ErrInvalidTag for %q, got %v", in, err)
		}
	}
}

func TestParseTagWhitespace(t *testing.T) {
	calls, err := parseTag("  required ,  min = 8 ")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if calls[0].name != "required" {
		t.Fatalf("name=%q", calls[0].name)
	}
	if calls[1].name != "min" || calls[1].param != "8" {
		t.Fatalf("got %+v", calls[1])
	}
}

func TestJSONTagName(t *testing.T) {
	for _, tc := range []struct {
		in, want string
	}{
		{"", ""},
		{"-", ""},
		{"name", "name"},
		{"name,omitempty", "name"},
		{",omitempty", ""},
	} {
		if got := jsonTagName(tc.in); got != tc.want {
			t.Fatalf("json(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}
