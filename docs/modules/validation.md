# Validation

> Laravel-style struct validation for Go: declare constraints with `validate:"..."` tags, validate in one call, get back a typed `Errors` slice with translated messages.

## Concepts

A `*validation.Validator` owns a registry of named rules and a `Translator` that turns each rule violation into a human message. `ValidateStruct(ctx, v)` walks the struct's exported fields, runs every tagged rule, and returns `nil` or an `Errors` slice. `ValidateField(ctx, value, tag)` checks a single value against an inline rule expression — useful for ad-hoc / dynamic checks outside structs.

```
Validator ── rule registry (31 built-ins + your AddRule)
   ├─ ValidateStruct(ctx, v) ── walks struct tags, recurses into nested structs
   ├─ ValidateField(ctx, value, tag) ── ad-hoc single-value check
   └─ Translator ── FieldError → human message ({field}/{tag}/{param}/{value})
```

## Validator API

| Method | Notes |
|---|---|
| `New() *Validator` | Validator pre-populated with built-in rules and the English translator |
| `ValidateStruct(ctx, value) error` | Validates struct tags; returns `Errors` (or wrapped `ErrNotStruct` / `ErrInvalidTag` / `ErrUnknownRule`) |
| `ValidateField(ctx, value, tag) error` | Validates one value against an inline tag expression |
| `AddRule(name, Rule) error` | Registers/overwrites a rule; invalidates the compiled-type cache |
| `HasRule(name) bool` / `Rules() []string` | Membership test / sorted rule names |
| `SetTranslator(Translator)` / `Translator() Translator` | Swap / read the active translator |

`Rule` is `func(rc RuleContext) error` — return `nil` to pass, any error to fail (the message is supplied by the `Translator`, not the rule). `RuleContext` carries `Ctx`, `Field`, `Tag`, `Value`, `Kind`, `Param`, and `Parent` (for cross-field rules).

## Quick start

```go
import (
    "context"

    "github.com/godx-jp/godx-platform-framework/framework"
    "github.com/godx-jp/godx-platform-framework/validation"
)

type Signup struct {
    Email    string `validate:"required,email,max=255"`
    Password string `validate:"required,min=8"`
    Age      int    `validate:"gte=18,lte=120"`
}

ctx := context.Background()
app := framework.New("svc", "1.0.0").Use(validation.Module)
_ = app.Init(ctx)
defer app.Shutdown(ctx)

v, _ := validation.FromApp(app)
if err := v.ValidateStruct(ctx, signup); err != nil {
    for _, fe := range err.(validation.Errors) {
        fmt.Println(fe.Field, fe.Message)
    }
}
```

## Built-in rules

| Rule        | Param       | Notes |
|-------------|-------------|-------|
| `required`  | —           | Non-zero value (length > 0 for strings / slices; non-zero for numbers) |
| `min`       | numeric     | Magnitude or length ≥ param |
| `max`       | numeric     | Magnitude or length ≤ param |
| `len`       | numeric     | Exact match |
| `between`   | `lo\|hi`    | Inclusive range |
| `eq` / `ne` | scalar      | Equality / non-equality; strings literal, numbers numeric |
| `gt` / `gte` / `lt` / `lte` | numeric | Numeric magnitude only |
| `in` / `oneof` | space- or pipe-separated whitelist | Compares stringified value |
| `email`     | —           | Strict `mail.ParseAddress` round-trip |
| `url`       | —           | Requires scheme + host |
| `uuid`      | —           | RFC 4122 canonical form |
| `regex`     | pattern     | Compiled once and cached |
| `ip`        | —           | IPv4 or IPv6 |
| `ipv4`      | —           | IPv4-only |
| `ipv6`      | —           | IPv6-only |
| `alpha`     | —           | `[a-zA-Z]+` |
| `numeric`   | —           | `[0-9]+` (string of digits) |
| `alphanum`  | —           | `[a-zA-Z0-9]+` |
| `json`      | —           | `encoding/json` parses cleanly |
| `startswith` / `endswith` / `contains` | substring | String operations |
| `eqfield` / `nefield` / `gtfield` / `ltfield` | sibling Go-field name | Cross-field comparison |

## Tag syntax

Rules are comma-separated. Parameters follow `=` and may be quoted with single quotes when they contain commas or equals signs:

```go
type Order struct {
    Status string `validate:"required,oneof='pending,confirmed,cancelled'"`
    Tags   []string `validate:"min=1,max=10"`
}
```

A leading or trailing comma — or two commas in a row — is rejected with `ErrInvalidTag` at compile time (first call to `ValidateStruct`). Unknown rule names also surface as `ErrUnknownRule` at compile time, not silently.

## Nullable semantics

Laravel-style: any field that is the zero value **and** not tagged `required` skips all subsequent rules. So `Bio string \`validate:"min=10"\`` accepts an empty string (don't validate when absent) and enforces `min=10` only when the field is set.

## Custom rules

```go
v, _ := validation.FromApp(app) // or v := validation.New()
_ = v.AddRule("notreserved", func(rc validation.RuleContext) error {
    for _, r := range strings.Split(rc.Param, "|") {
        if fmt.Sprint(rc.Value) == r {
            return fmt.Errorf("reserved")
        }
    }
    return nil
})

type Form struct {
    Name string `validate:"required,notreserved=root|admin"`
}
```

`AddRule` overwrites existing entries (built-ins included), so you can swap out behaviour for tests or special locales.

## Translations

The default `*MapTranslator` ships English templates for every built-in rule. Replace it or extend it:

```go
tr := validation.NewMapTranslator()
tr.Add("required", "{tag} là bắt buộc")
tr.Add("min", "{tag} cần ít nhất {param} ký tự")
tr.SetFallback("{tag} không hợp lệ ({rule})")
v.SetTranslator(tr)
```

Templates may use `{field}` (dotted path), `{tag}` (preferred display name — JSON tag falls back to the Go field name), `{rule}`, `{param}`, and `{value}` placeholders.

## Nested structs

Nested struct fields (value or pointer) are validated recursively. The dotted field path reflects the nesting (`Address.ZIP`). Nil pointer-to-struct fields skip recursion (Laravel-style: don't fail when the substructure is absent).

```go
type Address struct {
    ZIP string `validate:"required,len=5"`
}
type Customer struct {
    Name    string `validate:"required"`
    Address Address
}
```

## Error model

A failed `ValidateStruct` / `ValidateField` returns an `Errors` value (a `[]FieldError` that satisfies `error`). Each `FieldError` carries `Field` (dotted path), `Tag` (display name), `Rule`, `Param`, `Value`, and the translated `Message`.

```go
if err := v.ValidateStruct(ctx, signup); err != nil {
    var ve validation.Errors
    if errors.As(err, &ve) {
        for _, fe := range ve {
            fmt.Println(fe.Field, fe.Rule, fe.Message)
        }
    }
}
```

`Errors` exposes `Has()`, `HasField(field)`, `FieldErrors(field)`, `Add(fe)`, and `AsError()`. Malformed tags or unknown rule names are **programmer errors**, not validation failures, so they surface as the distinct sentinels `ErrInvalidTag`, `ErrUnknownRule`, `ErrNotStruct`, and `ErrInvalidParam` (test with `errors.Is`) rather than as entries in `Errors`.

## Context propagation

`validation.ContextWithValidator(ctx, v)` attaches a validator to a context; `validation.FromContext(ctx)` retrieves it (`ok == false` when none is present).

`validation.FromApp(app)` is the canonical way to retrieve the validator published by `validation.Module`; it returns an error when the module has not been initialised.

## Lifecycle

`validation.Module` publishes a single `*Validator` into the framework Store under `validation.StoreKey`. It registers no shutdown hook (the validator holds no external resources). Only one `validation.Module` may be wired per `App` — a second init returns an error. The validator is safe for concurrent use; share the one published by the module across the whole process.

## Environment-variable reference

The validation module reads no environment variables today — configuration happens through `Module` / `ModuleWithValidator(v)`.

## Laravel API mapping

| Laravel idiom                                             | Framework idiom                                |
|-----------------------------------------------------------|------------------------------------------------|
| `Validator::make($data, ['email' => 'required\|email'])`  | `v.ValidateField(ctx, value, "required,email")` or struct tags |
| `Request->validate(['name' => 'required'])`               | `v.ValidateStruct(ctx, req)`                   |
| `$rules['email'] = 'unique:users'`                        | `v.AddRule("unique", customRuleHittingDB)`     |
| `Lang::add(['en' => ['required' => '...']])`              | `tr.Add("required", "...")`                    |
| `Validator::extend('foo', fn …)`                          | `v.AddRule("foo", func)`                       |
