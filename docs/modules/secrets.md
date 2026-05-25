# Secrets

> Uniform `Get` / `Put` / `Forget` over environment variables, file
> mounts, HashiCorp Vault, Google Secret Manager, and AWS Secrets
> Manager. Same API in dev (env) and production (vault / cloud KMS).

## Concepts

The secrets module is a thin facade over five drivers. Applications fetch credentials by **logical key** (`db/password`, `auth/token`) and the active driver translates that key into a backend lookup. Switching backends — from local env-vars to cloud KMS — is a configuration change, no code change.

A `*secrets.Manager` owns one or more named stores. `Default()` returns the manager's default; `Store(name)` returns a specific one. The four-method `driver.Store` interface (`Get` / `Put` / `Forget` / `List`) is uniform; backends that cannot enumerate return `driver.ErrListNotSupported`, and read-only backends (env) return `driver.ErrReadOnly` for writes.

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

## Drivers

| Name      | Visibility    | Writable     | Listable | Notes |
|-----------|---------------|--------------|----------|-------|
| `env`     | auto          | no           | no       | reads `SECRETS_<KEY>` (upper-cased, dashes/dots/slashes → underscore) |
| `file`    | auto          | yes (atomic) | yes      | K8s-style; each key is one file under spec.Path |
| `vault`   | blank-import  | yes          | yes      | HashiCorp KV-v2 |
| `gcpsm`   | blank-import  | yes          | yes      | Google Cloud Secret Manager (ADC) |
| `awssm`   | blank-import  | yes          | yes      | AWS Secrets Manager (`SecretBinary` per key) |

Auto-registered drivers register themselves on package import (already wired by `import "…/secrets"`). Heavy drivers must be blank-imported so their cloud SDKs stay out of binaries that don't use them:

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

## Security notes

- Stores are not designed to *transform* secrets — they store and retrieve raw bytes. Pair with the [`encryption`](encryption.md) module if you need ciphertext-at-rest semantics on top of a backend that already encrypts at rest (e.g. KMS-backed values stored in a relational DB).
- The `file` driver writes 0600-mode files via atomic temp+rename to avoid partial reads.
- The `env` driver is intentionally read-only; production env vars are immutable from the process's perspective.
- The `vault` driver stores values base64-encoded under a `value` field of a KV-v2 secret, allowing binary payloads. Plain-string secrets written manually outside the driver remain readable verbatim.

## Migrating from go-common

`umbrella/packages/go-common` does not currently expose a secrets abstraction — services either read env vars directly or pull from AWS Secrets Manager through bespoke wrappers. Migration paths:

| go-common pattern                                 | Replacement                                     |
|---------------------------------------------------|--------------------------------------------------|
| `os.Getenv("DB_PASSWORD")` scattered through code | `mgr.GetString(ctx, "db/password")` once at startup |
| ad-hoc `secretsmanager.NewFromConfig(...)` clients| Module + `_ "…/secrets/drivers/awssm"` blank import |
| AWS SM look-ups inside hot paths                   | Read once at startup; bind to a typed struct |

The recommended migration:

1. Add `secrets.Module` to the service's framework App.
2. Replace scattered `os.Getenv` calls with `mgr.GetString` (the env driver is the default; behaviour is unchanged in dev).
3. In production, set `SECRETS_DEFAULT=awssm` + `SECRETS_AWSSM_REGION` and blank-import `…/drivers/awssm`; no code change.

## Integration tests

The heavy drivers (vault, gcpsm, awssm) ship with smoke / contract unit tests only. Live emulator coverage (LocalStack for AWS, the Vault test cluster, the GCP secret-manager test fixture) belongs to a follow-up integration suite tracked alongside the broader observability-emulator work.
