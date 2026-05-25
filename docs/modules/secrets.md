# Secrets

> Uniform `Get` / `Put` / `Forget` over environment variables, file
> mounts, HashiCorp Vault, Google Secret Manager, and AWS Secrets
> Manager. Same API in dev (env) and production (vault / cloud KMS).

## Concepts

The secrets module is a thin facade over five drivers. Applications fetch credentials by **logical key** (`db/password`, `auth/token`) and the active driver translates that key into a backend lookup. Switching backends — from local env-vars to cloud KMS — is a configuration change, no code change.

A `*secrets.Manager` owns one or more named stores; each store is backed by a driver chosen at deploy time. `Default()` returns the manager's default; `Store(name)` returns a specific one. The five-method `driver.Store` interface (`Name` / `Get` / `Put` / `Forget` / `List` plus `Shutdown`) is uniform; backends that cannot enumerate return `driver.ErrListNotSupported`, and read-only backends (env) return `driver.ErrReadOnly` for writes.

```
Manager ── named Stores
   └─ Store(name) ── Get / Put / Forget (Laravel-style facade)
         └─ driver.Store (env · file · vault · gcpsm · awssm)
```

## Quick start

```go
import (
    "context"

    "github.com/godx-jp/godx-platform-framework/framework"
    "github.com/godx-jp/godx-platform-framework/secrets"
    _ "github.com/godx-jp/godx-platform-framework/secrets/drivers/vault"
)

ctx := context.Background()
app := framework.New("svc", "1.0.0").Use(secrets.Module)
if err := app.Init(ctx); err != nil { /* … */ }
defer app.Shutdown(ctx)

mgr, _ := secrets.FromApp(app)
dbPass, err := mgr.GetString(ctx, "db/password")
```

`secrets.Module` reads its config from the environment by default; pass `secrets.ModuleWithConfig(cfg)` to wire it programmatically.

## Manager API

Every method takes `context.Context` first and operates on the **default** store. Use `Store(name)` to obtain a specific `driver.Store` and call its methods directly.

| Method | Notes |
|---|---|
| `Get(ctx, key) ([]byte, error)` | Reads from the default store; `driver.ErrNotFound` when absent |
| `GetString(ctx, key) (string, error)` | String wrapper around `Get` |
| `Put(ctx, key, value) error` | Writes to the default store; `driver.ErrReadOnly` on read-only backends |
| `PutString(ctx, key, value) error` | String wrapper around `Put` |
| `Forget(ctx, key) error` | Removes a key; a missing key is not an error |
| `Default() driver.Store` | The default store (nil when none registered) |
| `Store(name) (driver.Store, error)` | A named store |
| `Stores() []string` | Sorted list of registered store names |
| `AddStore(name, driver.Store) error` / `SetDefault(name) error` | Register a store / switch the default |
| `Shutdown(ctx) error` | Shuts every store down; joins per-store errors |

The `driver.Store` interface itself is `Name()`, `Get`, `Put`, `Forget`, `List(ctx) ([]string, error)`, and `Shutdown`. `List` returns `driver.ErrListNotSupported` on backends that cannot enumerate (env).

## Drivers

| Driver | Status | Registration | Writable | Listable | Notes |
|--------|--------|--------------|----------|----------|-------|
| `env`   | stable | auto | no           | no  | reads `<prefix><KEY>` (default prefix `SECRETS_`); key upper-cased, dashes/dots/slashes/spaces → underscore |
| `file`  | stable | auto | yes (atomic) | yes | K8s/Docker-style; each key is one file under `spec.Path` (sub-keys become directories), 0600 mode, trailing newline trimmed on read |
| `vault` | stable | opt-in (`_ "...drivers/vault"`) | yes | yes | HashiCorp Vault KV-v2 |
| `gcpsm` | stable | opt-in (`_ "...drivers/gcpsm"`) | yes | yes | Google Cloud Secret Manager (Application Default Credentials) |
| `awssm` | stable | opt-in (`_ "...drivers/awssm"`) | yes | yes | AWS Secrets Manager (`SecretBinary` per key) |

**Light** drivers (`env`, `file`) auto-register on `import "…/secrets"`. **Heavy** drivers must be blank-imported so their cloud SDKs stay out of binaries that don't use them:

```go
import (
    _ "github.com/godx-jp/godx-platform-framework/secrets/drivers/vault"
    _ "github.com/godx-jp/godx-platform-framework/secrets/drivers/gcpsm"
    _ "github.com/godx-jp/godx-platform-framework/secrets/drivers/awssm"
)
```

### Key normalisation

Logical keys are translated by each driver:

| Logical key      | env (default prefix)      | file (root `/etc/secrets`) | gcpsm (prefix `myapp`)             |
|------------------|---------------------------|----------------------------|------------------------------------|
| `db/password`    | `SECRETS_DB_PASSWORD`     | `/etc/secrets/db/password` | `projects/<p>/secrets/myapp-db-password` |
| `auth-token`     | `SECRETS_AUTH_TOKEN`      | `/etc/secrets/auth-token`  | `projects/<p>/secrets/myapp-auth-token`  |

(`awssm` uses slash-separated names verbatim — `myapp/db/password`.)

## Mixed-driver deployments

Register more than one store to read from several backends:

```go
cfg := secrets.Config{
    Default: "vault",
    Stores: map[string]secrets.StoreConfig{
        "vault": {Driver: "vault", Spec: sdriver.Spec{
            Name: "vault", Address: "https://vault:8200", Token: tok, Prefix: "myapp",
        }},
        "local": {Driver: "file", Spec: sdriver.Spec{
            Name: "file", Path: "/etc/secrets",
        }},
    },
}
app := framework.New("svc", "1.0.0").Use(secrets.ModuleWithConfig(cfg))
```

Then `mgr.Default()` returns the Vault store and `mgr.Store("local")` returns the file store.

## Environment-variable reference

See [docs/CONFIGURATION.md](../CONFIGURATION.md#secrets) for the canonical list. Quick summary:

| Variable                  | Default | Notes |
|---------------------------|---------|-------|
| `SECRETS_DEFAULT`         | `env`   | default store name |
| `SECRETS_STORES`          | (default only) | CSV of store names; each name infers its driver |
| `SECRETS_PREFIX`          | (none)  | global key prefix for all stores |
| `SECRETS_ENV_PREFIX`      | `SECRETS_` | env-driver-only override (use `-` for no prefix) |
| `SECRETS_FILE_PATH`       |         | file-driver root directory (required when file is enabled) |
| `SECRETS_VAULT_ADDR`      |         | Vault API endpoint |
| `SECRETS_VAULT_TOKEN`     |         | static Vault token (env also respects VAULT_TOKEN) |
| `SECRETS_VAULT_KV_MOUNT`  | `secret` | KV-v2 mount path |
| `SECRETS_GCPSM_PROJECT`   |         | GCP project id |
| `SECRETS_AWSSM_REGION`    |         | AWS region (overrides AWS_REGION) |

## Laravel API mapping

There is no canonical Laravel `Secrets` facade, but the API mirrors Laravel's `Config::get(...)` ergonomics for any backend:

| Laravel idiom                              | Framework idiom                    |
|--------------------------------------------|------------------------------------|
| `env('DB_PASSWORD')` (production usage)    | `mgr.GetString(ctx, "db/password")` |
| `config(['db.password' => $v])` (runtime)  | `mgr.Put(ctx, "db/password", v)`   |
| Spatie's `laravel-secrets` `Secret::get()` | `mgr.GetString(ctx, key)`          |

## Error model

All drivers return stable sentinels from `secrets/driver` — branch with `errors.Is`:

```go
v, err := mgr.Get(ctx, "db/password")
switch {
case errors.Is(err, driver.ErrNotFound):         // key absent
case errors.Is(err, driver.ErrReadOnly):          // backend refuses writes (env)
case errors.Is(err, driver.ErrListNotSupported):  // backend cannot enumerate (env)
case errors.Is(err, driver.ErrClosed):            // store used after Shutdown
case err != nil:                                  // backend failure
default:                                           // success
}
```

## Context propagation

`secrets.ContextWithManager(ctx, mgr)` attaches a manager to a context for handlers that prefer pulling from `context.Context` over a closure. `secrets.FromContext(ctx)` retrieves it (`ok == false` when none is present).

`secrets.FromApp(app)` is the canonical way to retrieve the manager built by `secrets.Module`; it returns an error when the module has not been initialised.

## Lifecycle

`secrets.Module` registers `Manager.Shutdown` via `app.OnShutdown`. On shutdown the manager walks every registered store and calls `Shutdown`, joining any per-store errors so a misbehaving backend does not block the rest. Only one `secrets.Module` may be wired per `App` — a second init returns an error.

## Security notes

- Stores are not designed to *transform* secrets — they store and retrieve raw bytes. Pair with the [`encryption`](encryption.md) module if you need ciphertext-at-rest semantics on top of a backend that already encrypts at rest (e.g. KMS-backed values stored in a relational DB).
- The `file` driver writes 0600-mode files via atomic temp+rename to avoid partial reads.
- The `file` driver confines all filesystem access to its root with `os.Root` (Go 1.24+): symlinks that resolve outside the root are not traversed, so a symlink planted on a shared volume cannot redirect `Get`/`List`/`Put` to an out-of-root target (e.g. `/etc/shadow`). Lexical `../` key rejection is kept as defense-in-depth.
- The `awssm` driver's `Forget` deletes secrets using AWS Secrets Manager's default recovery window (7–30 days) rather than `ForceDeleteWithoutRecovery`, so an accidental or injected delete can be reversed with `RestoreSecret`.
- The `env` driver is intentionally read-only; production env vars are immutable from the process's perspective.
- The `vault` driver stores values base64-encoded under a `value` field of a KV-v2 secret, allowing binary payloads. Plain-string secrets written manually outside the driver remain readable verbatim.

## Integration tests

The heavy drivers (vault, gcpsm, awssm) ship with smoke / contract unit tests only. Live emulator coverage (LocalStack for AWS, the Vault test cluster, the GCP secret-manager test fixture) belongs to a follow-up integration suite tracked alongside the broader observability-emulator work.
