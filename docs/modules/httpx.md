# HTTPX

> chi router helpers, a `HandlerFunc` convention with unified error
> handling, and a middleware layer that composes the framework's
> `pipeline`, `validation`, `ratelimit`, and `auth` modules into a
> net/http stack.

## Concepts

`httpx` is an HTTP composition layer, not a driver-backed module. There is no `httpx.Module`, no manager, and no config — you build a router and attach handlers and middleware directly. It sits on top of `github.com/go-chi/chi/v5` and re-exports the framework's cross-cutting modules as `func(http.Handler) http.Handler` middleware so a service wires everything through one import.

```
NewRouter() ── *chi.Mux (RequestID · RealIP · Recoverer)
   ├─ Route / Group ── HandlerFunc → Serve → unified error response
   └─ middleware/ ── auth · validation · ratelimit · pipeline
         (each adapts a framework module to func(http.Handler) http.Handler)
```

## Quick start

```go
package main

import (
    "net/http"

    "github.com/godx-jp/godx-platform-framework/httpx"
)

func main() {
    r := httpx.NewRouter()

    httpx.Route(r, http.MethodGet, "/ping", func(w http.ResponseWriter, req *http.Request) error {
        httpx.JSON(w, http.StatusOK, map[string]string{"pong": "1"})
        return nil
    })

    _ = http.ListenAndServe(":8080", r)
}
```

See [examples/httpx/main.go](../../examples/httpx/main.go).

## Router

`NewRouter(opts ...RouterOptions)` returns a `*chi.Mux`. With no options (or an all-false `RouterOptions`) it enables chi's `RequestID`, `RealIP`, and `Recoverer` middleware. Set any field on `RouterOptions` to opt in selectively — once any field is `true`, only the requested middleware are installed.

```go
type RouterOptions struct {
    RequestID bool // chi request-ID middleware
    RealIP    bool // chi RealIP middleware
    Recoverer bool // chi panic recoverer
}

r := httpx.NewRouter(httpx.RouterOptions{Recoverer: true}) // recoverer only
```

`Route(r chi.Router, method, pattern string, h HandlerFunc)` registers a handler via `r.Method`. `Group(r chi.Router, fn func(chi.Router), stages ...func(http.Handler) http.Handler)` builds a sub-router, applies the given middleware to it, then runs `fn` to register routes on the group.

## Handler convention

```go
type HandlerFunc func(w http.ResponseWriter, r *http.Request) error
```

Return `nil` on success or an error to delegate the response to `Serve`, which `Route` applies for you. To call a `HandlerFunc` outside `Route`, wrap it with `Serve(h) http.HandlerFunc`.

| Function | Purpose |
|---|---|
| `Serve(h HandlerFunc) http.HandlerFunc` | Adapt a `HandlerFunc`; writes the error response when it returns non-nil |
| `JSON(w, status int, v any)` | Write `v` as JSON with `application/json; charset=utf-8` |
| `NoContent(w, status int)` | Write a status code with an empty body |
| `DecodeJSON(r *http.Request, dst any) error` | Decode the body with `DisallowUnknownFields`; returns a 400 `*StatusError` on a nil body or decode failure |
| `NewStatusError(code int, message string) *StatusError` | Build a status-carrying error |
| `WrapStatus(code int, message string, err error) *StatusError` | Wrap a cause with a status |

## Error model

`StatusError` carries an HTTP status, message, and an optional wrapped cause. It implements `error` and `Unwrap`, so it composes with `errors.As` / `errors.Is`.

```go
type StatusError struct {
    Code    int
    Message string
    Err     error
}
```

`Serve` routes a handler's returned error through unified error handling:

- The error is unwrapped with `errors.As` to find a `*StatusError` with `Code > 0`.
- Status `>= 500` is written as a plain `http.Error` with the standard status text — the message is **not** leaked to the client.
- Status `< 500` is written as JSON `{"error": <message>}`.
- Any error that is not a `*StatusError` (or has `Code == 0`) becomes a plain `500 Internal Server Error`.

```go
func createUser(w http.ResponseWriter, r *http.Request) error {
    var dto CreateUser
    if err := httpx.DecodeJSON(r, &dto); err != nil {
        return err // already a 400 *StatusError
    }
    if dto.Email == "" {
        return httpx.NewStatusError(http.StatusUnprocessableEntity, "email required")
    }
    httpx.JSON(w, http.StatusCreated, dto)
    return nil
}
```

## RFC 9457 Problem Details

For APIs that expose a structured error envelope, configure the type URI prefix once at startup, then call `WriteProblem`:

```go
httpx.SetProblemTypeBaseURL("https://errors.example.com")

httpx.WriteProblem(w, r, "orders", http.StatusNotFound, "not-found", "Order not found")
httpx.WriteProblem(w, r, "orders", http.StatusUnprocessableEntity, "validation-failed", "invalid",
    httpx.WithErrors(httpx.FieldError{Field: "email", Code: "required", Message: "required"}),
)
```

When `SetProblemTypeBaseURL` is not called, `TypeURI` falls back to `urn:problem:{service}:{slug}`.

Mount Swagger UI before auth middleware: `httpx.MountOpenAPI(r, httpx.OpenAPIConfig{Title: "Orders API", Spec: specBytes})`.

## Middleware

The `httpx/middleware` package (imported here as `hmw`) adapts framework modules into the standard `func(http.Handler) http.Handler` shape.

```go
r := httpx.NewRouter()
r.Use(hmw.RateLimitByIP(limiter))
r.Use(hmw.Pipeline(loggingStage))

r.Group(func(sub chi.Router) {
    sub.Use(hmw.Authenticate(authMgr))
    sub.Use(hmw.ValidateJSON(v, func() any { return &CreateUser{} }))
    httpx.Route(sub, http.MethodPost, "/users", createUser)
})
```

### Composition & pipeline

| Function | Source module | Notes |
|---|---|---|
| `Stack(layers ...func(http.Handler) http.Handler) func(http.Handler) http.Handler` | — | Composes layers outermost-first; nil layers are skipped |
| `Pipeline(stages ...pipeline.HTTPStage) func(http.Handler) http.Handler` | `pipeline` | Wraps `pipeline.Chain`; first stage runs outermost |

### Auth (wraps `auth/middleware`)

| Function | Purpose |
|---|---|
| `Authenticate(mgr *auth.Manager, guardName ...string)` | rejects unauthenticated requests |
| `Optional(mgr *auth.Manager, guardName ...string)` | resolves an actor if present, never rejects |
| `RequireRole(roles ...string)` | require any of the given roles |
| `RequirePermission(perms ...string)` | require any of the given permissions |
| `RequireActorKind(kinds ...auth.ActorKind)` | restrict by actor kind |
| `RequireGate(name string)` | gate check by name |
| `RequireAuthorize(ability string, resource func(*http.Request) (any, error))` | policy authorization against a resolved resource |

### Validation (wraps `validation`)

`ValidateJSON(v *validation.Validator, factory func() any) func(http.Handler) http.Handler` decodes the body into a fresh DTO from `factory()`, runs `v.ValidateStruct`, and stores the value on the request context. It responds `400` (`invalid json`) on a decode failure and `422` (the validator's error text) on a validation failure. Handlers retrieve the validated DTO with the generic helper:

```go
dto, ok := middleware.Validated[*CreateUser](r)
```

### Rate limit (wraps `ratelimit/middleware`)

| Function | Notes |
|---|---|
| `RateLimit(l rdriver.Limiter, keyFunc rlmw.KeyFunc) func(http.Handler) http.Handler` | `RetryAfter` is fixed at `1s` |
| `RateLimitByIP(l rdriver.Limiter) func(http.Handler) http.Handler` | Convenience wrapper keyed by client IP (`rlmw.ByIP`) |

## Lifecycle

`httpx` holds no state and registers nothing on the framework `App`. The lifecycle of the modules it composes (auth managers, rate limiters, validators) belongs to those modules. Pass already-built dependencies into the middleware constructors.
