# Auth

> Named authentication guards (JWT, API key) with HTTP middleware for principals, roles, permissions, actor kinds, and authorization gates.

## Concepts

A `Manager` holds one or more named **guards**. Each guard is a driver that validates credentials and returns a `Principal` on the request context.

```
Manager ── named Guards (jwt · apikey · …)
   └─ Authenticate(ctx, cred) ── Principal in context
Gates ── auth.Define(name, fn) ── auth.Check(name, principal)
```

## Quick start

```go
app := framework.New("svc", "1.0.0").Use(auth.Module)
_ = app.Init(ctx)
mgr, _ := auth.FromApp(app)
p, err := mgr.Authenticate(ctx, &driver.CredentialRequest{Token: bearer})
```

Default guard is **apikey** when `AUTH_DEFAULT` is unset. Light drivers auto-register when you import the `auth` package.

## HTTP middleware

```go
import (
    "github.com/godx-jp/godx-platform-framework/auth/middleware"
)

r.Use(middleware.Authenticate(mgr, "jwt"))
r.With(middleware.RequireRole("admin")).Get("/admin", handler)
r.With(middleware.RequirePermission("posts:edit")).Patch("/posts/{id}", handler)
r.With(middleware.RequireActorKind(auth.ActorService)).Get("/internal/ping", handler)
r.With(middleware.RequireGate("manage-posts")).Delete("/posts/{id}", handler)
```

| Middleware | Status | When |
|------------|--------|------|
| `Authenticate(mgr, guard?)` | **401** | Credentials missing or invalid |
| `Optional(mgr, guard?)` | — | Continues without principal on auth failure |
| `RequireRole(roles...)` | **403** | Principal lacks any listed role |
| `RequirePermission(perms...)` | **403** | Principal lacks any listed permission |
| `RequireActorKind(kinds...)` | **403** | Principal actor kind not allowed |
| `RequireGate(name)` | **403** | `auth.Check(name, principal)` fails |

Responses are JSON: `{"error":"unauthenticated|forbidden","message":"..."}`.

When `guard` is omitted, middleware uses the manager's default guard.

### httpx wrapper

```go
import htmw "github.com/godx-jp/godx-platform-framework/httpx/middleware"

r.Use(htmw.Authenticate(mgr, "jwt"))
r.Use(htmw.RequireRole("admin"))
```

## Gates

Laravel-style authorization gates evaluate the authenticated principal:

```go
auth.MustDefine("manage-posts", func(p *auth.Principal) bool {
    return auth.HasPermission(p, "posts:manage") || auth.HasRole(p, "admin")
})

// In a handler after Authenticate middleware:
if auth.Check("manage-posts", principal) { /* allowed */ }
```

## Drivers

| Driver | Registration | Notes |
|--------|--------------|-------|
| `jwt` | auto | Bearer token verified against JWKS (RS*/ES*) |
| `apikey` | auto | Static key map via `Spec.Keys` or `AUTH_GUARD_*_KEYS` |
| `introspect` | blank import | OAuth2 introspection stub |

## Env vars

| Variable | Default | Purpose |
|----------|---------|---------|
| `AUTH_DEFAULT` | `apikey` | Default guard name |
| `AUTH_GUARDS` | default only | CSV of guard names |
| `AUTH_GUARD_<NAME>_DRIVER` | inferred | `jwt` · `apikey` · `introspect` |
| `AUTH_GUARD_<NAME>_JWKS_URL` | _unset_ | JWKS URL (jwt) |
| `AUTH_GUARD_<NAME>_ISSUER` | _unset_ | Required `iss` claim (jwt) |
| `AUTH_GUARD_<NAME>_AUDIENCE` | _unset_ | Required `aud` claim (jwt) |
| `AUTH_GUARD_<NAME>_ROLES_CLAIM` | `roles` | JWT claim for roles |
| `AUTH_GUARD_<NAME>_PERMISSIONS_CLAIM` | `permissions` | JWT claim for permissions |
| `AUTH_GUARD_<NAME>_SUBJECT_CLAIM` | `sub` | JWT subject claim |
| `AUTH_GUARD_<NAME>_ACTOR_KIND_CLAIM` | _unset_ | JWT claim for actor kind |
| `AUTH_GUARD_<NAME>_HEADER` | `X-API-Key` | API key header name |
| `AUTH_GUARD_<NAME>_KEYS` | _unset_ | CSV `subject:secret` entries |

Full list: [CONFIGURATION.md](../CONFIGURATION.md#auth).

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

## API reference

| Symbol | Description |
|--------|-------------|
| `mgr.Authenticate(ctx, cred)` | Run guard from credential request |
| `auth.PrincipalFromContext(ctx)` | Principal from request context |
| `auth.FromApp(app)` | Manager from framework store |
| `auth.Define / MustDefine / Check` | Register and evaluate gates |
| `auth.HasRole / HasPermission / HasActorKind` | Authorization helpers |

## Laravel mapping

| Laravel | Framework |
|---------|-----------|
| `Auth::guard('api')->user()` | `auth.PrincipalFromContext(r.Context())` |
| `middleware('auth:api')` | `middleware.Authenticate(mgr, "api")` |
| `middleware('can:edit-posts')` | `middleware.RequirePermission("edit-posts")` |
| `Gate::define('update', fn)` | `auth.Define("update", fn)` |
| `Gate::allows('update')` | `auth.Check("update", $user)` |
| `auth.php` guards array | `AUTH_GUARDS` + per-guard env vars |

## Tests

| File | Covers |
|------|--------|
| `middleware/http_test.go` | 401/403 semantics, JWT + apikey integration |
| `driver/registry_test.go` | Register / Lookup / New |
| `manager_test.go` | Manager wiring, gates |
| `module_test.go` | App wiring, env defaults |

```bash
go test -race ./auth/...
```

## Migrating from go-common

Replace ad-hoc JWT parsing and API-key checks with named guards on a shared `Manager`. Use **jwt** for user sessions and **apikey** for service credentials; enforce authorization with role/permission middleware and gates instead of inline conditionals.
