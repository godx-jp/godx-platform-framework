# Hashing

> Password hashing — Laravel's `Hash` facade reimagined for Go.
> Three modern drivers (`bcrypt`, `argon2id`, `scrypt`), self-describing
> encoded output, and an explicit `NeedsRehash` signal for work-factor
> upgrades.

## Concepts

A `Hasher` produces self-describing encoded hashes (`$2y$…`, `$argon2id$…`, `$scrypt$…`) so verification doesn't need separate parameters. A `Manager` holds one or more named hashers, so a service can run bcrypt for legacy users and argon2id for new signups side by side — and `Manager.CheckAny` will route an inbound encoded hash to whichever hasher understands it.

```
Manager ── named Hashers
   └─ Hasher(name)  ── Make / Check / NeedsRehash / Info
         └─ driver.Hasher (bcrypt · argon2id · scrypt)
```

## Quick start

```go
package main

import (
    "context"

    "github.com/godx-jp/godx-platform-framework/framework"
    "github.com/godx-jp/godx-platform-framework/hashing"
)

func main() {
    ctx := context.Background()
    app := framework.New("svc", "1.0.0").Use(hashing.Module)
    if err := app.Init(ctx); err != nil { panic(err) }
    defer app.Shutdown(ctx)

    mgr, _ := hashing.FromApp(app)
    h := mgr.Default()

    enc, _ := h.Make(ctx, "hunter2")
    ok,  _ := h.Check(ctx, "hunter2", enc)
    if h.NeedsRehash(enc) {
        enc, _ = h.Make(ctx, "hunter2")
    }
    _ = ok
}
```

With no env vars set you get a single `bcrypt` hasher at cost 12 — Laravel's default.

For tests and scripts that don't want a full App, `hashing.MustDefault()` returns a ready bcrypt hasher (cost 12) and panics on construction failure:

```go
h := hashing.MustDefault()
enc, _ := h.Make(ctx, "hunter2")
```

## Env-var config

| Var | Purpose | Default |
|---|---|---|
| `HASHING_DEFAULT` | Default hasher name | `bcrypt` |
| `HASHING_HASHERS` | Comma-separated list of hashers to register | _default hasher only_ |
| `HASHING_BCRYPT_COST` | bcrypt cost factor (4..31) | `12` |
| `HASHING_ARGON2ID_TIME` | argon2id iterations | `3` |
| `HASHING_ARGON2ID_MEMORY` | argon2id memory in KiB | `65536` (64 MiB) |
| `HASHING_ARGON2ID_THREADS` | argon2id parallelism | `2` |
| `HASHING_SCRYPT_N` | scrypt CPU/memory cost (power of 2) | `32768` |
| `HASHING_SCRYPT_R` | scrypt block size | `8` |
| `HASHING_SCRYPT_P` | scrypt parallelism | `1` |

Each name in `HASHING_HASHERS` becomes a registered hasher; the driver is inferred from the name (`bcrypt`/`argon2id`/`scrypt`, falling back to `bcrypt` for unknown names). Cost env vars apply to every hasher of the matching driver.

## Programmatic config

```go
import (
    "github.com/godx-jp/godx-platform-framework/framework"
    "github.com/godx-jp/godx-platform-framework/hashing"
    hdriver "github.com/godx-jp/godx-platform-framework/hashing/driver"
)

cfg := hashing.Config{
    Default: "primary",
    Hashers: map[string]hashing.HasherConfig{
        "primary": {Driver: "argon2id"},
        "legacy":  {Driver: "bcrypt", Spec: hdriver.Spec{Name: "bcrypt", BcryptCost: 12}},
    },
}
app := framework.New("svc", "1.0.0").Use(hashing.ModuleWithConfig(cfg))
```

`Config.Validate` requires `Default` to be non-empty, at least one hasher, the default name to be present in `Hashers`, and every hasher to name a driver. A `HasherConfig` with an empty `Spec.Name` inherits its `Driver` as the spec name at build time.

## Mixed-driver deployments

```go
mgr, _ := hashing.FromApp(app)
ok, name, _ := mgr.CheckAny(ctx, plain, storedHash)   // resolves driver from the encoded hash
```

`CheckAny` tries each registered hasher whose `Info` recognises the encoded hash, returning the matching hasher's name. It lets you accept either bcrypt or argon2id during a migration without changing the user's stored hash until the next successful login (at which point `NeedsRehash` will be true and the application can re-`Make` with the new default). When no registered hasher recognises the encoding, it returns an error.

## Hasher API

`driver.Hasher` is the per-algorithm contract; every method takes `context.Context` first (except `NeedsRehash`/`Info`, which are pure).

| Method | Laravel parallel |
|---|---|
| `Make(ctx, plain) (string, error)` | `Hash::make($plain)` — returns the self-describing encoded hash |
| `Check(ctx, plain, hash) (bool, error)` | `Hash::check($plain, $hash)` |
| `NeedsRehash(hash) bool` | `Hash::needsRehash($hash)` — true when the stored hash is weaker than current config |
| `Info(hash) (driver.Info, error)` | `Hash::info($hash)` — `{Algorithm, Params}`; `ErrUnknownFormat` if unrecognised |
| `Name() string` | the canonical driver name |

## Manager API

| Method | Notes |
|---|---|
| `mgr.Default() driver.Hasher` | the hasher flagged as default |
| `mgr.Hasher(name) (driver.Hasher, error)` | a specific named hasher; error when unregistered |
| `mgr.Hashers() []string` | sorted names of registered hashers |
| `mgr.AddHasher(name, h) error` | register a hasher; first registration becomes default; duplicate name errors |
| `mgr.SetDefault(name) error` | flag an already-registered hasher as default |
| `mgr.CheckAny(ctx, plain, hash) (bool, string, error)` | try every hasher; returns the matching hasher's name |
| `mgr.Shutdown(ctx) error` | no-op today (hashers hold no resources) |

## Driver matrix

| Driver | Status | Encoded prefix | Notes |
|---|---|---|---|
| `bcrypt`   | stable | `$2[ay]$…`      | Laravel default. 72-byte hard limit on plaintext. Cost 4..31 (default 12). |
| `argon2id` | stable | `$argon2id$v=…` | OWASP-recommended. Memory + time + threads cost, PHC string format. |
| `scrypt`   | stable | `$scrypt$…`     | RFC 7914. CPU/memory cost via `N` (power of 2), `r`, `p`. |

All three are **light** drivers — pure CPU (stdlib + `golang.org/x/crypto`), no network dependency — and auto-register via blank imports in the `hashing` package's `register.go`, so importing `hashing` makes all three available.

## Error model

The driver package exports typed sentinels (`github.com/godx-jp/godx-platform-framework/hashing/driver`):

```go
enc, err := h.Make(ctx, plain)
switch {
case errors.Is(err, driver.ErrPasswordTooLong):    // plaintext over the driver limit (bcrypt: 72 bytes)
case errors.Is(err, driver.ErrIncompatibleParams): // a Spec value out of range (e.g. bcrypt cost > 31)
}

ok, err := h.Check(ctx, plain, hash)
// ok == false with err == nil is a clean mismatch.
if errors.Is(err, driver.ErrInvalidHash) {          // the encoded hash is malformed
}

info, err := h.Info(hash)
if errors.Is(err, driver.ErrUnknownFormat) {         // this driver doesn't recognise the encoding — try CheckAny
}
```

| Sentinel | Meaning |
|---|---|
| `driver.ErrInvalidHash` | encoded hash is malformed (bad segment count / base64) |
| `driver.ErrUnknownFormat` | the hasher does not recognise this encoding |
| `driver.ErrPasswordTooLong` | plaintext exceeds the driver's hard limit |
| `driver.ErrIncompatibleParams` | a `Spec` parameter is outside the accepted range |

## Laravel API mapping

| Laravel | Framework |
|---|---|
| `Hash::make($plain)` | `h.Make(ctx, plain)` |
| `Hash::check($plain, $hash)` | `h.Check(ctx, plain, hash)` |
| `Hash::needsRehash($hash)` | `h.NeedsRehash(hash)` |
| `Hash::info($hash)` | `h.Info(hash)` |
| `Hash::driver('argon2id')` | `mgr.Hasher("argon2id")` |
| `config/hashing.php → driver` | `HASHING_DEFAULT` env var |

## Context propagation

`hashing.ContextWithManager(ctx, mgr)` attaches a `*Manager` to a context; `hashing.FromContext(ctx)` retrieves it (`ok == false` when absent). `hashing.FromApp(app)` is the canonical way to retrieve the manager built by `hashing.Module`.

## Lifecycle

`hashing.Module` registers `Manager.Shutdown` as an `OnShutdown` callback. `Shutdown` is a no-op today — hashers hold no resources — but is present so the module fits framework lifecycle conventions. Only one `hashing.Module` may be initialised per App.

## Out of scope

- **Password policy** — strength checks (length, dictionary, breached) belong in the `validation` module.
- **Symmetric encryption** — the `encryption` module. Don't reach for hashing for "store and later read"; use encryption.
- **Token signing / JWT** — separate concern; integrate via your auth layer.
