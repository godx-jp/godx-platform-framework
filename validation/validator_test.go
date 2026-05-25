package validation

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type signup struct {
	Email    string `json:"email" validate:"required,email,max=255"`
	Password string `json:"password" validate:"required,min=8"`
	Age      int    `json:"age" validate:"gte=18,lte=120"`
}

func TestValidateStructHappyPath(t *testing.T) {
	v := New()
	s := signup{Email: "a@b.com", Password: "abcdefgh", Age: 30}
	if err := v.ValidateStruct(context.Background(), s); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestValidateStructFailures(t *testing.T) {
	v := New()
	s := signup{Email: "bad", Password: "short", Age: 5}
	err := v.ValidateStruct(context.Background(), s)
	if err == nil {
		t.Fatalf("expected errors")
	}
	errs, ok := err.(Errors)
	if !ok {
		t.Fatalf("err type=%T", err)
	}
	if len(errs) < 3 {
		t.Fatalf("expected >=3 errors, got %d (%v)", len(errs), errs)
	}
	if !errs.HasField("Email") || !errs.HasField("Password") || !errs.HasField("Age") {
		t.Fatalf("missing per-field err: %v", errs)
	}
	for _, fe := range errs.FieldErrors("Email") {
		if fe.Rule == "email" && !strings.Contains(fe.Message, "email") {
			t.Fatalf("email rule msg missing word: %q", fe.Message)
		}
	}
}

func TestValidateStructNullableSkipped(t *testing.T) {
	type tag struct {
		Bio string `validate:"min=10"`
	}
	v := New()
	// empty string → not required → min check is skipped.
	if err := v.ValidateStruct(context.Background(), tag{}); err != nil {
		t.Fatalf("nullable failed: %v", err)
	}
	// non-empty triggers min.
	if err := v.ValidateStruct(context.Background(), tag{Bio: "short"}); err == nil {
		t.Fatalf("expected failure")
	}
}

func TestValidateStructPointer(t *testing.T) {
	v := New()
	s := &signup{Email: "a@b.com", Password: "abcdefgh", Age: 30}
	if err := v.ValidateStruct(context.Background(), s); err != nil {
		t.Fatalf("ptr: %v", err)
	}
}

func TestValidateStructNotStruct(t *testing.T) {
	v := New()
	err := v.ValidateStruct(context.Background(), 5)
	if !errors.Is(err, ErrNotStruct) {
		t.Fatalf("err=%v", err)
	}
	err = v.ValidateStruct(context.Background(), nil)
	if !errors.Is(err, ErrNotStruct) {
		t.Fatalf("nil err=%v", err)
	}
}

func TestUnknownRuleFailsCompile(t *testing.T) {
	type bad struct {
		Foo string `validate:"reqd"`
	}
	v := New()
	err := v.ValidateStruct(context.Background(), bad{})
	if !errors.Is(err, ErrUnknownRule) {
		t.Fatalf("err=%v", err)
	}
}

func TestNestedStructRecursed(t *testing.T) {
	type addr struct {
		Zip string `validate:"required,len=5"`
	}
	type person struct {
		Name    string `validate:"required"`
		Address addr
	}
	v := New()
	err := v.ValidateStruct(context.Background(), person{Name: "alice", Address: addr{Zip: "1"}})
	if err == nil {
		t.Fatalf("expected nested err")
	}
	errs := err.(Errors)
	if !errs.HasField("Address.Zip") {
		t.Fatalf("nested field path wrong: %v", errs)
	}
}

func TestPointerNestedStructNilSkipped(t *testing.T) {
	type addr struct {
		Zip string `validate:"required"`
	}
	type person struct {
		Address *addr
	}
	v := New()
	if err := v.ValidateStruct(context.Background(), person{}); err != nil {
		t.Fatalf("nil ptr: %v", err)
	}
}

func TestCustomRule(t *testing.T) {
	v := New()
	if err := v.AddRule("notfoo", func(rc RuleContext) error {
		if s, ok := stringValue(rc.Value, rc.Kind); ok && s == "foo" {
			return errors.New("nope")
		}
		return nil
	}); err != nil {
		t.Fatalf("AddRule: %v", err)
	}
	if !v.HasRule("notfoo") {
		t.Fatalf("HasRule false")
	}
	type x struct {
		S string `validate:"notfoo"`
	}
	if err := v.ValidateStruct(context.Background(), x{S: "foo"}); err == nil {
		t.Fatalf("expected err")
	}
	if err := v.ValidateStruct(context.Background(), x{S: "bar"}); err != nil {
		t.Fatalf("bar should pass: %v", err)
	}
}

func TestAddRuleRejectsEmptyOrNil(t *testing.T) {
	v := New()
	if err := v.AddRule("", func(rc RuleContext) error { return nil }); err == nil {
		t.Fatalf("empty should error")
	}
	if err := v.AddRule("x", nil); err == nil {
		t.Fatalf("nil should error")
	}
}

func TestValidateFieldStandalone(t *testing.T) {
	v := New()
	if err := v.ValidateField(context.Background(), "a@b.com", "required,email"); err != nil {
		t.Fatalf("standalone: %v", err)
	}
	if err := v.ValidateField(context.Background(), "nope", "email"); err == nil {
		t.Fatalf("expected email err")
	}
}

func TestRulesSorted(t *testing.T) {
	v := New()
	rules := v.Rules()
	for i := 1; i < len(rules); i++ {
		if rules[i-1] > rules[i] {
			t.Fatalf("not sorted: %v", rules)
		}
	}
}

func TestCustomTranslator(t *testing.T) {
	v := New()
	tr := NewMapTranslator()
	tr.Add("required", "VN: {tag} là bắt buộc")
	tr.SetFallback("VN: lỗi {rule}")
	v.SetTranslator(tr)
	err := v.ValidateStruct(context.Background(), signup{})
	errs := err.(Errors)
	for _, fe := range errs {
		if fe.Rule == "required" && !strings.Contains(fe.Message, "bắt buộc") {
			t.Fatalf("custom translator not applied: %q", fe.Message)
		}
	}
}

func TestSetTranslatorNilIgnored(t *testing.T) {
	v := New()
	before := v.Translator()
	v.SetTranslator(nil)
	if v.Translator() != before {
		t.Fatalf("nil translator should be ignored")
	}
}

func TestErrorsHelpers(t *testing.T) {
	e := Errors{
		{Field: "a"}, {Field: "b"}, {Field: "a"},
	}
	if !e.Has() || !e.HasField("a") || e.HasField("z") {
		t.Fatalf("helpers wrong: %v", e)
	}
	if got := len(e.FieldErrors("a")); got != 2 {
		t.Fatalf("FieldErrors a=%d", got)
	}
	if e.AsError() == nil {
		t.Fatalf("AsError nil")
	}
	empty := Errors{}
	if empty.AsError() != nil {
		t.Fatalf("empty AsError not nil")
	}
}
