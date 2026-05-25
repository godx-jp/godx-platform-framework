# Validation

> Laravel-style struct validation for Go: declare constraints with `validate:"..."` tags, validate in one call, get back a typed `Errors` slice with translated messages.

## Concepts

A `*validation.Validator` owns a registry of named rules and a `Translator` that turns each rule violation into a human message. `ValidateStruct(ctx, v)` walks the struct's exported fields, runs every tagged rule, and returns `nil` or an `Errors` slice. `ValidateField(ctx, value, tag)` checks a single value against an inline rule expression — useful for ad-hoc / dynamic checks outside structs.

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
v := validation.FromApp(app) // or validation.New()
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

## Migrating from go-common

`umbrella/packages/go-common` exposes a few ad-hoc validators (email regex, uuid parser) but no struct-tag framework. Migration path:

1. Add `validation.Module` to the service's framework App.
2. Add `validate:"…"` tags to request DTOs.
3. Replace per-field `if request.Email == "" { … }` blocks with one `v.ValidateStruct(ctx, request)`.
4. For ad-hoc checks (e.g. validating a string from a URL query parameter), use `v.ValidateField(ctx, value, "required,uuid")`.

The validator is safe for concurrent use; share one `*Validator` (typically the one published by `validation.Module`) across the whole process.
