# Hashing

> Password hashing — Laravel's `Hash` facade reimagined for Go.
> Three modern drivers (`bcrypt`, `argon2id`, `scrypt`), self-describing
> encoded output, and an explicit `NeedsRehash` signal for work-factor
> upgrades.

## Concepts

A `Hasher` produces self-describing encoded hashes (`$2y$…`, `$argon2id$…`, `$scrypt$…`) so verification doesn't need separate parameters. A `Manager` holds one or more named hashers, so a service can run bcrypt for legacy users and argon2id for new signups side by side — and `Manager.CheckAny` will route an inbound encoded hash to whichever Hasher understands it.

```
Manager ── named Hashers
   └─ Hasher(name)  ── Make / Check / NeedsRehash / Info
         └─ driver.Hasher (bcrypt · argon2id · scrypt)
```

## Quick start

```go
import (
    "github.com/godx-jp/godx-platform-framework/framework"
    "github.com/godx-jp/godx-platform-framework/hashing"
)

app := framework.New("svc", "1.0.0").Use(hashing.Module)
_ = app.Init(ctx)

mgr, _ := hashing.FromApp(app)
h := mgr.Default()

enc, _ := h.Make(ctx, "hunter2")
ok,  _ := h.Check(ctx, "hunter2", enc)
if h.NeedsRehash(enc) {
    enc, _ = h.Make(ctx, "hunter2")
}
```

With no env vars set you get a single `bcrypt` hasher at cost 12 — Laravel's default.

## Drivers

| Driver | Status | Encoded prefix | Notes |
|---|---|---|---|
| `bcrypt`   | stable | `$2[ay]$…`     | Laravel default. 72-byte hard limit on plaintext. Cost 4..31 (default 12). |
| `argon2id` | stable | `$argon2id$v=…` | OWASP 2024 default. Memory + time + threads cost. PHC string format. |
| `scrypt`   | stable | `$scrypt$ln=…` | RFC 7914. CPU/memory cost via `N` (power of 2), `r`, `p`. |

All three are light drivers (no network deps) and auto-register when the `hashing` package is imported.

## Mixed-driver deployments

```go
cfg := hashing.Config{
    Default: "primary",
    Hashers: map[string]hashing.HasherConfig{
        "primary": {Driver: "argon2id"},
        "legacy":  {Driver: "bcrypt", Spec: hashing.driver.Spec{
            Name: "bcrypt", BcryptCost: 12,
        }},
    },
}
app := framework.New(...).Use(hashing.ModuleWithConfig(cfg))

mgr, _ := hashing.FromApp(app)
ok, name, _ := mgr.CheckAny(ctx, plain, storedHash)   // resolves driver from prefix
```

`CheckAny` lets you accept either bcrypt or argon2id during a migration without changing the user's stored hash until the next successful login (at which point `NeedsRehash` will be true and the application can re-Make with the new default).

## Env var reference

| Var | Purpose | Default |
|---|---|---|
| `HASHING_DEFAULT` | Default driver name | `bcrypt` |
| `HASHING_HASHERS` | Comma-separated list of hashers to register | _default driver only_ |
| `HASHING_BCRYPT_COST` | Cost factor (4..31) | `12` |
| `HASHING_ARGON2ID_TIME` | Iterations | `3` |
| `HASHING_ARGON2ID_MEMORY` | Memory in KiB | `65536` (64 MiB) |
| `HASHING_ARGON2ID_THREADS` | Parallelism | `2` |
| `HASHING_SCRYPT_N` | CPU/memory cost (power of 2) | `32768` |
| `HASHING_SCRYPT_R` | Block size | `8` |
| `HASHING_SCRYPT_P` | Parallelism | `1` |

## Laravel API mapping

| Laravel | Framework |
|---|---|
| `Hash::make($plain)` | `h.Make(ctx, plain)` |
| `Hash::check($plain, $hash)` | `h.Check(ctx, plain, hash)` |
| `Hash::needsRehash($hash)` | `h.NeedsRehash(hash)` |
| `Hash::info($hash)` | `h.Info(hash)` |
| `Hash::driver('argon2id')` | `mgr.Hasher("argon2id")` |
| `config/hashing.php → driver` | `HASHING_DEFAULT` env var |

## Migrating from go-common

`umbrella/packages/go-common` has per-service `golang.org/x/crypto/bcrypt` direct calls scattered through identity and platform handlers. Replace them:

| Before | After |
|---|---|
| `bcrypt.GenerateFromPassword([]byte(p), 12)` | `h.Make(ctx, p)` |
| `bcrypt.CompareHashAndPassword(stored, []byte(p))` | `h.Check(ctx, p, string(stored))` |
| Ad-hoc cost constant | `HASHING_BCRYPT_COST` env var, one place |
| Hand-coded rehash check | `h.NeedsRehash(stored)` |

Migration is incremental — pull `hashing.FromApp(app)` into the request scope; nothing else has to change at the call site.

## Out of scope

- **Password policy** — strength checks (length, dictionary, breached) belong in the upcoming `validation` module (v0.9.0).
- **Symmetric encryption** — `encryption` module (v0.8.3). Don't reach for hashing for "store and later read"; use encryption.
- **Token signing / JWT** — separate concern; integrate via your auth layer.
