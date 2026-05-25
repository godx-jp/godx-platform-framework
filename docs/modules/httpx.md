# HTTPX

> chi router helpers, a `HandlerFunc` convention with unified error
> handling, and a middleware layer that composes the framework's
> `pipeline`, `validation`, `ratelimit`, and `auth` modules into a
> net/http stack.

## Concepts

`httpx` is an HTTP composition layer, not a driver-backed module. There is no `httpx.Module`, no manager, and no config — you build a router and attach handlers and middleware directly. It sits on top of `github.com/go-chi/chi/v5` and re-exports the framework's cross-cutting modules as `func(http.Handler) http.Handler` middleware so a service wires everything through one import.

```
NewRouter() ── *chi.Mux (RequestID · Recoverer; RealIP opt-in only)
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

`NewRouter(opts ...RouterOptions)` returns a `*chi.Mux`. With no options it installs a safe default set of `RequestID` + `Recoverer` only. Pass a `RouterOptions` to choose middleware explicitly — exactly the fields set to `true` are installed.

```go
type RouterOptions struct {
    RequestID bool // chi request-ID middleware
    RealIP    bool // chi RealIP middleware (opt-in; trusted-proxy only)
    Recoverer bool // chi panic recoverer
}

r := httpx.NewRouter(httpx.RouterOptions{Recoverer: true}) // recoverer only
```

> **Security — `RealIP` is not enabled by default.** chi's `RealIP` rewrites `r.RemoteAddr` from the client-supplied `X-Forwarded-For` / `X-Real-IP` headers. Those headers are trivially spoofed, so trusting them at an internet-facing edge lets clients forge their own IP and defeats any `RemoteAddr`-based rate limiting (see `ratelimit`). Enable `RealIP: true` **only** when the service sits behind a trusted reverse proxy / load balancer that overwrites those headers.

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
| `DecodeJSON(r *http.Request, dst any) error` | Decode the body with `DisallowUnknownFields`, capped at `DefaultMaxBodyBytes`; returns a 400 `*StatusError` on a nil body or decode failure, or a 413 `*StatusError` when the body exceeds the cap |
| `DecodeJSONLimit(r *http.Request, dst any, limit int64) error` | Same as `DecodeJSON` with an explicit body-size cap (`limit <= 0` uses the default) |
| `NewStatusError(code int, message string) *StatusError` | Build a status-carrying error |
| `WrapStatus(code int, message string, err error) *StatusError` | Wrap a cause with a status |

## Request body-size limit

`DecodeJSON` wraps the request body in `http.MaxBytesReader` before decoding, capping it at `DefaultMaxBodyBytes` (1 MiB). This prevents a memory-exhaustion DoS from unbounded JSON payloads. When the body exceeds the cap the decoder returns an `*http.MaxBytesError`, which `DecodeJSON` maps to a **413 Request Entity Too Large** `*StatusError` — so `Serve` writes a 413, never a 500. Malformed bodies still map to 400.

```go
const DefaultMaxBodyBytes int64 = 1 << 20 // 1 MiB

// Override per call (e.g. a small webhook endpoint):
if err := httpx.DecodeJSONLimit(r, &dto, 64<<10); err != nil { // 64 KiB
    return err
}
```

The `ValidateJSON` middleware applies the same cap before decoding.

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

## Recommended middleware stack

Outer → inner on a chi router:

```
observability/middleware.HTTP   # trace + correlation + access log
  → httpx/middleware.Recover
  → httpx/middleware.RequestID  # X-Request-ID
  → httpx/middleware.RateLimit  # optional, per route group
  → auth/middleware.Authenticate
  → handler
```

| `httpx/middleware` | Purpose |
|---|---|
| `Recover()` | Panic recovery with structured log |
| `RequestID()` / `RequestIDFrom(ctx)` | `X-Request-ID` propagation |
| `RateLimit(l, keyFn)` / `RateLimitByIP(l)` | RFC 9110 429 + Retry-After |

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

`ValidateJSON(v *validation.Validator, factory func() any) func(http.Handler) http.Handler` decodes the body into a fresh DTO from `factory()` (capped at `DefaultMaxBodyBytes`), runs `v.ValidateStruct`, and stores the value on the request context.

Responses are safe by construction — the middleware never echoes raw decoder or validator error text:

- **413** when the body exceeds the size cap.
- **400** `{"error":"invalid json"}` on a malformed body or unknown field.
- **422** with a stable, structured envelope on a validation failure: a generic `error` plus a `fields` array of `{field, code, param?}`, where `code` is the machine-readable rule name (`required`, `email`, …) and `field` is the public DTO field name. No Go error strings or struct internals are leaked.

```json
{
  "error": "validation failed",
  "fields": [{ "field": "Email", "code": "email" }]
}
```

Handlers retrieve the validated DTO with the generic helper:

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
