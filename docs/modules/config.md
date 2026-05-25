# Config

> Laravel-faithful layered configuration repository. Compose values
> from any number of sources (env vars, files, remote KV systems);
> the application talks to one typed facade and never imports a
> backend.

## Concepts

A single `Manager` owns a chain of `Source` drivers. Each `Source.Load` returns a tree (`map[string]any`); the Manager merges them in registration order — **later sources win** — and publishes a `Repository` with typed accessors. The Repository pointer is stable across reloads; the underlying data is swapped in place.

```
Manager ── ordered chain of Sources
   └─ Repository  ── typed read/write facade
         └─ driver.Source (env · file · static · remote)
```

The implicit auto-env source (`__auto_env__`, opt-out via `CONFIG_AUTO_ENV=false`) is appended after every configured source so process environment always overrides file contents — the same precedence Laravel applies between `.env` and `config/*.php`.

## Quick start

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/godx-jp/godx-platform-framework/config"
    "github.com/godx-jp/godx-platform-framework/framework"
)

func main() {
    ctx := context.Background()
    app := framework.New("svc", "1.0.0").Use(config.Module)
    if err := app.Init(ctx); err != nil { log.Fatal(err) }
    defer app.Shutdown(ctx)

    cfg, _ := config.FromApp(app)
    port    := cfg.GetInt("server.port", 8080)
    timeout := config.Get[time.Duration](cfg, "http.timeout", 2*time.Second)
    _ = port
    _ = timeout
}
```

With nothing in the environment, the module wires only the auto-env source — useful for the smallest services.

## Layered sources via env

```bash
CONFIG_SOURCES=defaults,site
CONFIG_AUTO_ENV=true
CONFIG_ENV_PREFIX=MYAPP_           # only MYAPP_* env vars participate

CONFIG_SOURCE_DEFAULTS_DRIVER=file
CONFIG_SOURCE_DEFAULTS_PATH=./config/defaults.yaml

CONFIG_SOURCE_SITE_DRIVER=file
CONFIG_SOURCE_SITE_PATH=./config/site.toml
CONFIG_SOURCE_SITE_OPTIONAL=true   # missing file → empty tree
```

Merge precedence: `defaults` < `site` < env. The driver-name shortcut also works — `CONFIG_SOURCES=file` resolves to `CONFIG_SOURCE_FILE_DRIVER=file` so very small services don't need to repeat the name.

## Programmatic config

```go
cfg := config.Config{
    AutoEnv:       true,
    AutoEnvPrefix: "MYAPP_",
    Sources: []config.NamedSourceConfig{
        {Name: "defaults", Config: config.SourceConfig{
            Driver: "file", Path: "./config/defaults.yaml",
        }},
    },
}
app := framework.New(...).Use(config.ModuleWithConfig(cfg))
```

## Typed accessors

| Method | Behaviour |
|---|---|
| `Get(key) (any, bool)` | Raw value + presence |
| `GetString(key, def)` | Stringified bool/int/float fall back to `def` for other types |
| `GetInt(key, def)` | int64; parses strings via `strconv`; truncates floats |
| `GetFloat(key, def)` | float64 |
| `GetBool(key, def)` | accepts `1/0`, `true/false`, `yes/no`, `on/off` |
| `GetDuration(key, def)` | `time.ParseDuration`; numerics treated as ns |
| `GetSlice(key, def)` | `[]any` |
| `GetStringSlice(key, def)` | best-effort conversion |
| `GetMap(key, def)` | `map[string]any` |
| `Has(key)` | presence check |
| `Set(key, value)` | sets nested, creates intermediates |
| `Forget(key)` | removes |
| `All()` | full tree (shared storage) |
| `AllFlat()` | dot-flattened copy |
| `config.Get[T](repo, key, def)` | generic — wraps the typed helpers |

Keys are dot-separated. Empty segments are dropped (`a..b` → `a.b`).

## Drivers

| Driver | Source | Type | Notes |
|---|---|---|---|
| `env` | `config/drivers/env` | light, auto | Strips `Spec.Prefix`, lowercases keys, splits `__` to dots |
| `file` | `config/drivers/file` | light, auto | YAML, JSON, TOML (extension or explicit `Format`); optional `Watcher` polls mtime every 1s |
| `static` | `config/drivers/static` | light, auto | In-process map, primarily for tests and compile-time defaults |
| `remote` | `config/drivers/remote/*` | heavy, opt-in | Reserved for etcd / consul / vault implementations (not yet shipped) |

Light drivers register themselves via `init()`; heavy drivers require a blank import in the consumer's `main` package.

## Hot reload

The file driver implements `Watcher` — pass it through and Manager will call `Reload` automatically on file changes (mtime poll, 1s interval, no fsnotify dep). Register a callback for "rebuild caches" work:

```go
mgr, _ := config.ManagerFromApp(app)
mgr.OnChange(func(r *config.Repository) {
    log.Println("config reloaded")
})
```

## Env var reference

| Var | Purpose | Default |
|---|---|---|
| `CONFIG_SOURCES` | Ordered, comma-separated source names | _empty_ |
| `CONFIG_AUTO_ENV` | Append implicit env source after the chain | `true` |
| `CONFIG_ENV_PREFIX` | Filter the implicit env source | _empty_ (every var) |
| `CONFIG_SOURCE_<NAME>_DRIVER` | Driver name | inferred from name when possible, else `file` |
| `CONFIG_SOURCE_<NAME>_PATH` | File source path | _empty_ |
| `CONFIG_SOURCE_<NAME>_FORMAT` | Override file format (`yaml`\|`json`\|`toml`) | _inferred from extension_ |
| `CONFIG_SOURCE_<NAME>_OPTIONAL` | Missing file → empty tree | `false` |
| `CONFIG_SOURCE_<NAME>_PREFIX` | Driver-specific prefix scope | _empty_ |
| `CONFIG_SOURCE_<NAME>_URL` | Remote source URL | _empty_ |
| `CONFIG_SOURCE_<NAME>_ADDRESS` | Remote source `host:port` | _empty_ |
| `CONFIG_SOURCE_<NAME>_TOKEN` | Remote source auth token | _empty_ |

## Laravel API mapping

| Laravel | Framework |
|---|---|
| `Config::get('app.name', 'default')` | `repo.Get("app.name")` / `repo.GetString("app.name", "default")` |
| `Config::string / int / bool / array` | `repo.GetString / GetInt / GetBool / GetSlice` |
| `Config::set('app.name', 'X')` | `repo.Set("app.name", "X")` |
| `Config::has('app.name')` | `repo.Has("app.name")` |
| `Config::all()` | `repo.All()` / `repo.AllFlat()` |
| `Artisan config:cache` | `repo.AllFlat()` — emit to JSON for boot-time caching |

## Migrating from go-common

`umbrella/packages/go-common` exposes ad-hoc env wrappers (per-service helpers). Replace each call site:

| Before | After |
|---|---|
| `os.Getenv("HTTP_PORT")` with default | `cfg.GetString("http.port", "8080")` after setting `CONFIG_ENV_PREFIX=` to capture `HTTP_*` |
| Per-service Viper / koanf wrappers | `config.Module` with one or more file sources |
| Hand-rolled `.env` loader | Drop the loader, set `CONFIG_AUTO_ENV=true` (default) |

No big-bang rewrite — services can adopt the config module incrementally; the env source covers existing `MYAPP_FOO=bar` conventions out of the box.

## Out of scope

- **Schema validation** — handled by the upcoming `validation` module (`v0.9.0`). The Repository stays untyped on purpose so the same tree feeds many typed views.
- **Secrets handling** — handled by the upcoming `secrets` module (`v0.8.5`). Treat the config tree as non-secret by default.
- **Config encryption** — handled by the upcoming `encryption` module (`v0.8.3`). Sources can decrypt on Load if they want, but the Repository sees plaintext only.
