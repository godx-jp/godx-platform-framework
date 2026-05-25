// Run with `go run ./examples/validation` from the repo root.
//
// Demonstrates the Validator with three scenarios:
//
//  1. happy-path Signup that passes every rule.
//  2. failing Signup with three independent rule violations.
//  3. custom-rule registration + a Vietnamese translator.
package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/godx-jp/godx-platform-framework/framework"
	"github.com/godx-jp/godx-platform-framework/validation"
)

type Signup struct {
	Email    string `json:"email"    validate:"required,email,max=255"`
	Password string `json:"password" validate:"required,min=8"`
	Confirm  string `json:"confirm"  validate:"eqfield=Password"`
	Age      int    `json:"age"      validate:"gte=18,lte=120"`
	Role     string `json:"role"     validate:"oneof=admin|member|guest"`
}

func main() {
	ctx := context.Background()
	app := framework.New("validation-example", "0.0.0").Use(validation.Module)
	if err := app.Init(ctx); err != nil {
		log.Fatalf("init: %v", err)
	}
	defer func() { _ = app.Shutdown(ctx) }()

	v, _ := validation.FromApp(app)

	good := Signup{
		Email: "a@b.com", Password: "abcdefgh", Confirm: "abcdefgh",
		Age: 30, Role: "member",
	}
	must(v.ValidateStruct(ctx, good) == nil)
	fmt.Println("good signup: PASS")

	bad := Signup{Email: "nope", Password: "short", Confirm: "mismatch", Age: 5, Role: "ceo"}
	if err := v.ValidateStruct(ctx, bad); err != nil {
		errs := err.(validation.Errors)
		fmt.Printf("bad signup: %d violations\n", len(errs))
		for _, fe := range errs {
			fmt.Printf("  - %s\n", fe)
		}
	}

	fmt.Println()
	fmt.Println("--- custom rule + Vietnamese translator ---")
	custom := validation.New()
	tr := validation.NewMapTranslator()
	tr.Add("required", "{tag} là bắt buộc")
	tr.Add("min", "{tag} cần ít nhất {param} ký tự")
	tr.Add("email", "{tag} phải là email hợp lệ")
	tr.SetFallback("{tag} không hợp lệ ({rule})")
	custom.SetTranslator(tr)
	_ = custom.AddRule("notreserved", func(rc validation.RuleContext) error {
		if rc.Value == nil {
			return nil
		}
		v := fmt.Sprint(rc.Value)
		for _, r := range strings.Split(rc.Param, "|") {
			if v == r {
				return fmt.Errorf("reserved")
			}
		}
		return nil
	})
	type form struct {
		Name string `json:"name" validate:"required,min=3,notreserved=root|admin"`
	}
	if err := custom.ValidateStruct(ctx, form{Name: "admin"}); err != nil {
		for _, fe := range err.(validation.Errors) {
			fmt.Printf("  - %s\n", fe)
		}
	}
}

func must(ok bool) {
	if !ok {
		log.Fatalf("expected pass")
	}
}
