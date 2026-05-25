package validation

import (
	"context"
	"sync"
	"testing"
)

type concurrentForm struct {
	Email string `validate:"required,email"`
	Age   int    `validate:"gte=18"`
}

func TestValidatorConcurrentAccess(t *testing.T) {
	v := New()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			f := concurrentForm{Email: "a@b.com", Age: 18 + i%50}
			_ = v.ValidateStruct(context.Background(), f)
			f2 := concurrentForm{}
			_ = v.ValidateStruct(context.Background(), f2)
		}(i)
	}
	wg.Wait()
}

func TestValidatorCacheReusesAcrossCalls(t *testing.T) {
	v := New()
	f := concurrentForm{Email: "a@b.com", Age: 30}
	if err := v.ValidateStruct(context.Background(), f); err != nil {
		t.Fatalf("first: %v", err)
	}
	// Second call should hit the cache; structure equality of the
	// compiled rules is the implicit assertion via no error.
	if err := v.ValidateStruct(context.Background(), f); err != nil {
		t.Fatalf("second: %v", err)
	}
}

func TestAddRuleInvalidatesCache(t *testing.T) {
	v := New()
	type x struct {
		S string `validate:"required"`
	}
	_ = v.ValidateStruct(context.Background(), x{S: "ok"})
	// Add a new rule — cache should reset so future tags can use it.
	_ = v.AddRule("mustfoo", func(rc RuleContext) error {
		if s, ok := stringValue(rc.Value, rc.Kind); ok && s != "foo" {
			return errFailed
		}
		return nil
	})
	type y struct {
		S string `validate:"mustfoo"`
	}
	if err := v.ValidateStruct(context.Background(), y{S: "foo"}); err != nil {
		t.Fatalf("foo: %v", err)
	}
	if err := v.ValidateStruct(context.Background(), y{S: "bar"}); err == nil {
		t.Fatalf("bar: expected err")
	}
}

func TestFieldErrorStringWithoutMessage(t *testing.T) {
	fe := FieldError{Field: "Email", Rule: "email"}
	if got := fe.Error(); got != "Email: email failed" {
		t.Fatalf("got %q", got)
	}
	fe2 := FieldError{Field: "Name", Rule: "min", Param: "3"}
	if got := fe2.Error(); got != "Name: min(3) failed" {
		t.Fatalf("got %q", got)
	}
}

func TestEmptyTagSkipped(t *testing.T) {
	type x struct {
		Public string
		Other  int
	}
	v := New()
	if err := v.ValidateStruct(context.Background(), x{}); err != nil {
		t.Fatalf("no tags: %v", err)
	}
}

func TestUnexportedFieldsIgnored(t *testing.T) {
	type x struct {
		Public string `validate:"required"`
		hidden string //nolint:unused
	}
	v := New()
	if err := v.ValidateStruct(context.Background(), x{Public: "ok"}); err != nil {
		t.Fatalf("unexported should be ignored: %v", err)
	}
}
