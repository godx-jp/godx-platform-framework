// Package validation implements a Laravel-style struct validator —
// declare constraints with `validate:"..."` tags, validate a value
// in one call, get back a typed Errors slice with per-field detail
// and translated messages.
//
//	type Signup struct {
//	    Email    string `validate:"required,email,max=255"`
//	    Password string `validate:"required,min=8"`
//	    Age      int    `validate:"gte=18,lte=120"`
//	}
//	v := validation.New()
//	if errs := v.ValidateStruct(ctx, signup); errs != nil {
//	    for _, fe := range errs.(validation.Errors) {
//	        fmt.Println(fe.Field, fe.Message)
//	    }
//	}
//
// Rule registry. Built-in rules cover the common cases (required,
// length / magnitude bounds, whitelists, formats, comparisons,
// cross-field references). Register custom rules with AddRule.
//
// i18n. Each rule emits a structured FieldError carrying its name
// and parameter — the Translator turns it into a human message.
// Bundled English translations live in translator.go; supply
// your own Translator for additional locales.
//
// The validator is safe for concurrent use after construction.
package validation
