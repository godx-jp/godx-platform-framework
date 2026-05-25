# Auth

> Named authentication guards (JWT, API key) plus a Laravel-style
> authorization layer — gates, typed policies, and before/after hooks.
> The application talks to one `Manager` and a small set of
> context/middleware helpers; the backend is an env-var change.

## Concepts

A `Manager` holds one or more named **guards**. Each guard is a driver that validates a `CredentialRequest` and returns a `Principal`. A per-guard **credential resolver** extracts those credentials from an `*http.Request`. Authorization is a separate concern: process-global **gates** (`auth.Define`) and typed **policies** (`auth.RegisterPolicy`) are evaluated by `auth.Authorize`, with optional `Before`/`After` hooks.

```
Manager ── named Guards (jwt · apikey · introspect)
   │           └─ Authenticate(ctx, *CredentialRequest) → *Principal
   └─ per-guard CredentialResolver (HTTP → CredentialRequest)

Gates    ── auth.Define(name, fn)            ─┐
Policies ── auth.RegisterPolicy[T](policy)    ├─ auth.Authorize(ability, p, args…)
Hooks    ── auth.Before / auth.After          ─┘
```

`Principal`, `ActorKind`, and the actor-kind constants are type aliases re-exported from the `auth/driver` package, so application code only ever imports `auth`.

## Quick start

```go
package main

import (
    "context"

    "github.com/godx-jp/godx-platform-framework/auth"
    "github.com/godx-jp/godx-platform-framework/auth/driver"
    "github.com/godx-jp/godx-platform-framework/framework"
)

func main() {
    ctx := context.Background()
    app := framework.New("svc", "1.0.0").Use(auth.Module)
    if err := app.Init(ctx); err != nil { panic(err) }
    defer app.Shutdown(ctx)

    mgr, _ := auth.FromApp(app)
    p, err := mgr.Authenticate(ctx, &driver.CredentialRequest{APIKey: "internal-svc-secret"})
    _ = p; _ = err
}
```

With nothing in the environment you get a single `apikey` guard, also used as the default (`AUTH_DEFAULT` falls back to `apikey`).

## Guards via env

```bash
AUTH_DEFAULT=jwt
AUTH_GUARDS=jwt,apikey

# jwt guard (Bearer token verified against JWKS)
AUTH_GUARD_JWT_JWKS_URL=https://idp.example.com/.well-known/jwks.json
AUTH_GUARD_JWT_ISSUER=https://idp.example.com
AUTH_GUARD_JWT_AUDIENCE=my-api

# apikey guard (static key map)
AUTH_GUARD_APIKEY_KEYS=internal-svc:sekret,reporting:other-secret
```

The driver-name shortcut works the same way the other modules do: an entry in `AUTH_GUARDS` whose name is `jwt`, `apikey`, or `introspect` infers its own driver, so `AUTH_GUARDS=jwt` needs no `AUTH_GUARD_JWT_DRIVER=jwt`.

> The module wires a default credential resolver for each guard based on its driver: `jwt` → `Authorization: Bearer`, `apikey` → the configured header. `ResolverForGuard` only knows `jwt` and `apikey`, so configuring an `introspect` guard through the env module currently fails at `Init` with `no default resolver for driver "introspect"` — wire that guard programmatically with your own resolver instead.

## Programmatic config

```go
cfg := auth.Config{
    Default: "apikey",
    Guards: map[string]auth.GuardConfig{
        "apikey": {Driver: "apikey", Spec: driver.Spec{
            Header: "X-API-Key",
            Keys: map[string]driver.APIKeyEntry{
                "internal-svc": {SubjectID: "internal-svc", Secret: "sekret",
                    ActorKind: auth.ActorService, Roles: []string{"internal"}},
            },
        }},
    },
}
app := framework.New(...).Use(auth.ModuleWithConfig(cfg))
```

## HTTP middleware

```go
import "github.com/godx-jp/godx-platform-framework/auth/middleware"

r.Use(middleware.Authenticate(mgr, "jwt"))
r.With(middleware.RequireRole("admin")).Get("/admin", handler)
r.With(middleware.RequirePermission("posts:edit")).Patch("/posts/{id}", handler)
r.With(middleware.RequireActorKind(auth.ActorService)).Get("/internal/ping", handler)
r.With(middleware.RequireGate("manage-posts")).Delete("/posts/{id}", handler)
r.With(middleware.RequireAuthorize("update", loadPost)).Put("/posts/{id}", handler)
```

| Middleware | On failure | When |
|------------|------------|------|
| `Authenticate(mgr, guard…)` | **401** | Credentials missing or invalid |
| `Optional(mgr, guard…)` | — | Continues without a principal on auth failure |
| `RequireRole(roles…)` | **403** | Principal lacks every listed role |
| `RequirePermission(perms…)` | **403** | Principal lacks every listed permission |
| `RequireActorKind(kinds…)` | **403** | Principal actor kind not in the list |
| `RequireGate(name)` | **403** | `auth.Authorize(name, principal)` returns false |
| `RequireAuthorize(ability, resource)` | **403** | `auth.Authorize(ability, p, resource)` returns false; the `resource func(*http.Request) (any, error)` loads the target |

`guardName` is variadic on `Authenticate`/`Optional`; omit it to use the manager's default guard. Failure responses are JSON `{"error":"unauthenticated"|"forbidden","message":"…"}` (the `message` field is omitted when empty).

### httpx wrapper

`httpx/middleware` re-exports the same constructors so apps already using the httpx stack don't import two middleware packages:

```go
import htmw "github.com/godx-jp/godx-platform-framework/httpx/middleware"

r.Use(htmw.Authenticate(mgr, "jwt"))
r.Use(htmw.RequireRole("admin"))
```

## Authorization: gates, policies, hooks

Gates evaluate the principal alone; policies authorize an *ability* against a typed *resource*. `auth.Authorize(ability, p, args…)` is the single entry point: with no resource arg it consults the gate registry; with a resource it consults the policy registered for that type, falling back to a gate of the same name.

```go
// Gate — principal only.
auth.MustDefine("manage-posts", func(p *auth.Principal) bool {
    return auth.HasPermission(p, "posts:manage") || auth.HasRole(p, "admin")
})

// Policy func — one ability for a resource type.
_ = auth.RegisterPolicyFunc("update", func(p *auth.Principal, post Post) bool {
    return post.AuthorID == p.SubjectID
})

// Full Policy — many abilities for a resource type.
_ = auth.RegisterPolicy[Post](postPolicy{})

// Global hooks (Laravel Gate::before / Gate::after).
auth.Before(func(p *auth.Principal, ability string, args ...any) *bool {
    if auth.HasRole(p, "super-admin") { allow := true; return &allow }
    return nil // defer
})

if auth.Authorize("update", principal, post) { /* allowed */ }
```

A `Before` hook that returns a non-nil `*bool` short-circuits; an `After` hook can flip an allow to a deny. The policy dispatcher matches both `T` and `*T` resources. `auth.Check` is a backward-compatible alias for `auth.Authorize`. `ResetHooks` and `ResetPolicies` clear the global registries for tests.

## Driver matrix

| Driver | Status | Registration | Notes |
|--------|--------|--------------|-------|
| `apikey` | stable | auto | Static key map via `Spec.Keys` or `AUTH_GUARD_<NAME>_KEYS`. Per-key roles/permissions/actor-kind via `AUTH_GUARD_<NAME>_KEY_<SUBJECT>_*`. Light |
| `jwt` | stable | auto | Bearer token verified against a JWKS endpoint (RS*/ES*). Claims mapped via the `*_CLAIM` env vars. Light |
| `introspect` | stub | opt-in (`_ "...auth/drivers/introspect"`) | OAuth2 token-introspection placeholder; `Authenticate` returns `driver.ErrNotImplemented`. Heavy/not-shipped |

**Light** drivers (`apikey`, `jwt`) auto-register on `import "...auth"`. The **`introspect`** driver is *not* in the package's blank-import set — it registers only when explicitly imported, and even then is an unimplemented stub:

```go
import _ "github.com/godx-jp/godx-platform-framework/auth/drivers/introspect"
```

Selecting a driver that hasn't been registered fails at module init with a hint naming the missing import path.

## Use cases

### JWT user API (JWKS)

```bash
AUTH_GUARDS=jwt
AUTH_DEFAULT=jwt
AUTH_GUARD_JWT_JWKS_URL=https://idp.example.com/.well-known/jwks.json
AUTH_GUARD_JWT_ISSUER=https://idp.example.com
AUTH_GUARD_JWT_AUDIENCE=my-api
```

### Service-to-service API key

```bash
AUTH_GUARDS=apikey
AUTH_GUARD_APIKEY_KEYS=internal-svc:sekret
AUTH_GUARD_APIKEY_KEY_INTERNAL_SVC_ACTOR_KIND=service
AUTH_GUARD_APIKEY_KEY_INTERNAL_SVC_ROLES=internal
```

```go
r.Use(middleware.Authenticate(mgr, "apikey"))
r.Use(middleware.RequireActorKind(auth.ActorService))
```

### Dual guards with chi

```go
r.Route("/v1", func(r chi.Router) {
    r.Use(middleware.Authenticate(mgr, "jwt"))
})
r.Route("/internal", func(r chi.Router) {
    r.Use(middleware.Authenticate(mgr, "apikey"))
})
```

## Env var reference

| Variable | Default | Purpose |
|----------|---------|---------|
| `AUTH_DEFAULT` | `apikey` | Default guard name |
| `AUTH_GUARDS` | the default only | CSV of guard names |
| `AUTH_GUARD_<NAME>_DRIVER` | inferred from name | `jwt` · `apikey` · `introspect` |
| `AUTH_GUARD_<NAME>_JWKS_URL` | _unset_ | JWKS URL (jwt) |
| `AUTH_GUARD_<NAME>_ISSUER` | _unset_ | Required `iss` claim (jwt) |
| `AUTH_GUARD_<NAME>_AUDIENCE` | _unset_ | Required `aud` claim (jwt) |
| `AUTH_GUARD_<NAME>_ROLES_CLAIM` | `roles` | JWT claim carrying roles |
| `AUTH_GUARD_<NAME>_PERMISSIONS_CLAIM` | `permissions` | JWT claim carrying permissions |
| `AUTH_GUARD_<NAME>_SUBJECT_CLAIM` | `sub` | JWT subject claim |
| `AUTH_GUARD_<NAME>_ACTOR_KIND_CLAIM` | _unset_ | JWT claim carrying actor kind |
| `AUTH_GUARD_<NAME>_HEADER` | `X-API-Key` | API-key header name (apikey) |
| `AUTH_GUARD_<NAME>_KEYS` | _unset_ | CSV of `subject:secret` entries (apikey) |
| `AUTH_GUARD_<NAME>_INTROSPECT_URL` | _unset_ | Introspection endpoint (introspect) |
| `AUTH_GUARD_<NAME>_KEY_<SUBJECT>_ROLES` | _unset_ | Per-key roles CSV (apikey) |
| `AUTH_GUARD_<NAME>_KEY_<SUBJECT>_PERMISSIONS` | _unset_ | Per-key permissions CSV (apikey) |
| `AUTH_GUARD_<NAME>_KEY_<SUBJECT>_ACTOR_KIND` | _unset_ | Per-key actor kind (apikey) |

`<NAME>` and `<SUBJECT>` are uppercased with `-` mapped to `_`.

## API reference

| Symbol | Description |
|--------|-------------|
| `mgr.Authenticate(ctx, *CredentialRequest)` | Run the named (or default) guard |
| `mgr.Guard(name)` / `mgr.Default()` / `mgr.DefaultName()` | Look up guards |
| `mgr.AddGuard / SetDefault / SetResolver / Resolver / Names` | Wiring and introspection |
| `auth.FromApp(app)` | Manager from the framework store |
| `auth.PrincipalFromContext(ctx)` / `auth.PrincipalForGuard(ctx, guard)` | Principal from context |
| `auth.SubjectIDFromContext` / `auth.UserIDFromContext` | Subject id (the latter is an alias) |
| `auth.HasRole / HasPermission / HasActorKind` | Principal predicates |
| `auth.Define / MustDefine / GateNames` | Gate registry |
| `auth.RegisterPolicy[T] / RegisterPolicyFunc[T] / MustRegisterPolicy[T]` | Policy registry |
| `auth.Authorize / Check / Before / After` | Authorization evaluation and hooks |
| `auth.BearerTokenResolver / APIKeyHeaderResolver / ChainResolver / GuardResolver / ResolverForGuard` | Credential resolvers |
| `auth/testing.ActingAs / ActingAsContext` | Test helpers (Laravel `actingAs` parity) |

## Error model

Guards return `driver.ErrInvalidCredential` for missing/invalid credentials, `driver.ErrClosed` after `Shutdown`, and `driver.ErrNotImplemented` for the `introspect` stub. The HTTP middleware translates an authentication failure to **401** (`unauthenticated`) and an authorization failure to **403** (`forbidden`).

```go
p, err := mgr.Authenticate(ctx, cred)
if errors.Is(err, driver.ErrInvalidCredential) {
    // bad/missing token — surface 401
}
```

## HMAC guard (symmetric service JWT)

Use the `hmac` driver for closed service-to-service auth with a shared HS256 secret (RFC 7519 + RFC 6750 Bearer). Prefer the `jwt` guard (JWKS / RS256) for user-facing APIs and public IdP integration.

```bash
AUTH_GUARDS=service
AUTH_DEFAULT=service
AUTH_GUARD_SERVICE_DRIVER=hmac
AUTH_GUARD_SERVICE_SECRET=your-32-byte-or-longer-shared-secret
AUTH_GUARD_SERVICE_AUDIENCE=orders-service
AUTH_GUARD_SERVICE_LEEWAY_SECONDS=30
```

Programmatic guard:

```go
mgr := auth.NewManager()
g, _ := driver.New(ctx, driver.Spec{
    Name: driver.DriverHMAC,
    Secret: os.Getenv("INTER_SERVICE_SECRET"),
    Audience: "orders-service",
})
_ = mgr.AddGuard("service", g)
_ = mgr.SetResolver("service", auth.BearerTokenResolver())
```

Custom JWT claims land in `Principal.Claims`. Read them with `auth.ClaimString(p, "tenant_id")` or map access.

### Token lifetime validation

The `hmac` guard hardens its time-claim checks so a malformed or non-expiring token cannot slip through:

- **`exp` is mandatory.** A token without an `exp` claim is rejected (the parser runs with `WithExpirationRequired`). An `exp` that is present but not a number — e.g. string-encoded — is treated as **invalid**, never as "absent".
- **`nbf` is honoured.** If a `nbf` (not-before) claim is present and lies in the future beyond the configured leeway, the token is rejected — mirroring the existing `iat` leeway check.
- **`MaxTokenTTL` is enforced.** When the guard's `Spec.MaxTokenTTL` is greater than zero and `exp − iat` exceeds it, the token is rejected even if it is otherwise unexpired. Set it to cap how long any single token may live:

```bash
AUTH_GUARD_SERVICE_LEEWAY_SECONDS=30
```

```go
g, _ := driver.New(ctx, driver.Spec{
    Name:        driver.DriverHMAC,
    Secret:      os.Getenv("INTER_SERVICE_SECRET"),
    Audience:    "orders-service",
    MaxTokenTTL: 5 * time.Minute, // reject tokens minted with a longer lifetime
})
```

All of these failures surface as `driver.ErrInvalidCredential`, i.e. **401** through the middleware.

Dev/test token minting (not for production hot paths):

```go
import "github.com/godx-jp/godx-platform-framework/auth/token"

tok, err := token.IssueHS256(token.HMACOptions{
    Secret: []byte(secret), Issuer: "caller", Audience: "orders-service", Subject: "caller",
    Claims: map[string]any{"tenant_id": "t1"}, TTL: time.Minute,
})
```

## Context propagation

`auth.ContextWithPrincipal(ctx, p)` attaches the principal (and, when `p.Guard` is set, also binds it under that guard via `ContextWithPrincipalForGuard`). `PrincipalFromContext` / `PrincipalForGuard` read it back. `auth.ContextWithManager` / `auth.FromContext` carry the `*Manager` itself for handlers that prefer pulling it from context. `auth.FromApp(app)` is the canonical way to retrieve the manager built by `auth.Module`.

The `auth/testing` package injects a principal without HTTP credentials — `ActingAs(r, guard, p)` for `*http.Request`, `ActingAsContext(ctx, guard, p)` for non-HTTP tests.

## Lifecycle

`auth.Module` registers an `OnShutdown` callback that calls `Shutdown` on every guard (errors joined), releasing JWKS refreshers and other backend resources. Only one `auth.Module` per `App`; a second `Init` returns `auth: Module already initialised`.

## Laravel mapping

| Laravel | Framework |
|---------|-----------|
| `Auth::guard('api')->user()` | `auth.PrincipalFromContext(r.Context())` |
| `middleware('auth:api')` | `middleware.Authenticate(mgr, "api")` |
| `middleware('can:edit-posts')` | `middleware.RequirePermission("edit-posts")` |
| `Gate::define('update', fn)` | `auth.Define("update", fn)` |
| `Gate::allows('update', $post)` | `auth.Authorize("update", principal, post)` |
| `Gate::before / Gate::after` | `auth.Before / auth.After` |
| `policy()` / `AuthServiceProvider::$policies` | `auth.RegisterPolicy[T]` / `auth.RegisterPolicyFunc[T]` |
| `actingAs($user)` | `auth/testing.ActingAs` |
| `auth.php` guards array | `AUTH_GUARDS` + per-guard env vars |
