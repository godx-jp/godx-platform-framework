# Feature flags

> Pluggable feature-flag evaluation — Laravel feature parity for Go.
> An `Evaluator` resolves one flag for a user (plus optional attributes)
> through a swappable `Provider`, with optional per-evaluation in-memory
> caching.

## Concepts

A single `Evaluator` wraps one `driver.Provider`. The provider is chosen at deploy time: the default `config` driver reads boolean keys from the [config](config.md) module; heavy drivers (`openfeature`, `launchdarkly`, `unleash`, `flagsmith`) talk to external flag platforms. Application code only ever calls `Evaluator.Enabled` — swapping providers is an env-var change. An optional in-memory cache memoises results per `(flag,user,attrs)` for a TTL.

```
Evaluator
   ├─ optional per-eval cache (flag|user|attrs → bool, TTL)
   └─ driver.Provider (config · openfeature · launchdarkly · unleash · flagsmith)
```

## Quick start

```go
package main

import (
    "context"

    "github.com/godx-jp/godx-platform-framework/config"
    "github.com/godx-jp/godx-platform-framework/featureflag"
    "github.com/godx-jp/godx-platform-framework/framework"
)

func main() {
    ctx := context.Background()
    app := framework.New("svc", "1.0.0").
        Use(config.Module).        // config driver needs the config module
        Use(featureflag.Module)
    if err := app.Init(ctx); err != nil { panic(err) }
    defer app.Shutdown(ctx)

    eval, _ := featureflag.FromApp(app)
    if ok, _ := eval.Enabled(ctx, "new-checkout", "alice@example.com", map[string]any{"tier": "pro"}); ok {
        // new path
    }
}
```

The default driver is `config`. When `featureflag.Module` is initialised with the `config` driver and no `*config.Repository` is supplied, it pulls one from `config.FromApp(app)` — so the [config](config.md) module must be on the App, otherwise init fails with `featureflag: config driver requires config.Module`.

## Env-var config

| Variable | Default | Purpose |
|---|---|---|
| `FEATUREFLAG_DRIVER` | `config` | Provider driver name |
| `FEATUREFLAG_PREFIX` | `flags` | Config-driver key prefix |
| `FEATUREFLAG_CACHE` | `false` | Enable per-evaluation in-memory cache |
| `FEATUREFLAG_CACHE_TTL` | `1m` | Cache entry TTL (Go duration; must be positive) |
| `FEATUREFLAG_SDK_KEY` | _empty_ | SDK key for `launchdarkly` / `flagsmith` |
| `FEATUREFLAG_ENDPOINT` | _empty_ | Endpoint URL for `openfeature` / `unleash` (and optional for `flagsmith`) |
| `FEATUREFLAG_PROJECT` | _empty_ | Project identifier (heavy drivers) |
| `FEATUREFLAG_APP_NAME` | _empty_ | Application name (used by `unleash`) |

## Programmatic config

```go
cfg := featureflag.Config{
    Driver:   "config",
    Prefix:   "flags",
    Cache:    true,
    CacheTTL: 30 * time.Second,
}
repo, _ := config.FromApp(app)                       // required when Driver == "config"
app := framework.New("svc", "1.0.0").
    Use(config.Module).
    Use(featureflag.ModuleWithConfig(cfg, repo))
```

`ModuleWithConfig(cfg, repo)` takes an explicit `*config.Repository`; pass `nil` and the module resolves one from the App when the driver is `config`. For heavy drivers the repo is irrelevant — set the SDK-key/endpoint fields on `Config` instead. `Config.Validate` requires a non-empty `Driver` and a positive `CacheTTL` (both defaulted by `withDefaults`).

You can also build an `Evaluator` directly:

```go
eval, _ := featureflag.NewEvaluator(featureflag.EvaluatorOptions{
    Provider:     provider,    // a driver.Provider
    CacheEnabled: true,
    CacheTTL:     time.Minute, // <= 0 defaults to 1m
})
```

## API

| Method | Laravel parallel |
|---|---|
| `eval.Enabled(ctx, flag, user, attrs map[string]any) (bool, error)` | `Feature::active($flag, $user)` |
| `eval.Shutdown(ctx) error` | — closes the underlying provider |
| `featureflag.FromApp(app) (*Evaluator, error)` | resolve the evaluator built by the module |

`Enabled` returns `featureflag: flag name is required` when `flag` is empty. With caching enabled, a hit returns the memoised value without touching the provider; a miss calls `provider.Enabled` and stores the result for `CacheTTL`. The cache key is `flag|user` when `attrs` is empty, otherwise `flag|user|<attrs>`.

## Driver matrix

| Driver | Status | Registration | Notes |
|---|---|---|---|
| `config` | stable | auto (light) | Reads `<prefix>.<flag>` bool from the config module; falls back to a `<prefix>.<flag>.users` allowlist. Requires `config.Module` |
| `openfeature` | stub | opt-in (heavy) | Validates `FEATUREFLAG_ENDPOINT`; `Enabled` returns `driver.ErrNotConfigured` |
| `launchdarkly` | stub | opt-in (heavy) | Validates `FEATUREFLAG_SDK_KEY`; `Enabled` returns `driver.ErrNotConfigured` |
| `unleash` | stub | opt-in (heavy) | Validates `FEATUREFLAG_ENDPOINT` (uses `FEATUREFLAG_APP_NAME`); `Enabled` returns `driver.ErrNotConfigured` |
| `flagsmith` | stub | opt-in (heavy) | Validates `FEATUREFLAG_SDK_KEY` (optional `FEATUREFLAG_ENDPOINT`); `Enabled` returns `driver.ErrNotConfigured` |

**Light** — only the `config` driver auto-registers (it is imported by the package's `register.go`).

**Heavy** — the four SDK-backed drivers are **stubs today**: their constructors validate that the required connection field is present, but `Enabled` always returns `driver.ErrNotConfigured`. They register only on an explicit blank import:

```go
import _ "github.com/godx-jp/godx-platform-framework/featureflag/drivers/launchdarkly"
```

Selecting a driver that has not been imported fails at module init with a message naming the missing import path.

## Config driver keys

The config driver looks up `<prefix>.<flag>` (prefix defaults to `flags`). A `true` boolean enables the flag for everyone. Otherwise it consults `<prefix>.<flag>.users` — either a string slice or a comma-separated string — and enables the flag when the `user` argument is in that allowlist.

```yaml
flags:
  new-checkout: true            # on for everyone
  beta-ui:
    users:                      # on only for these users
      - alice@example.com
      - bob@example.com
```

```go
eval.Enabled(ctx, "beta-ui", "alice@example.com", nil) // → true
eval.Enabled(ctx, "beta-ui", "carol@example.com", nil) // → false
```

## Error model

```go
ok, err := eval.Enabled(ctx, flag, user, attrs)
```

- Empty `flag` → `featureflag: flag name is required`.
- A provider that has been shut down returns `driver.ErrClosed`.
- The heavy stub providers return `driver.ErrNotConfigured` from `Enabled`.
- `driver.ErrUnknownDriver` and the "not registered" init error guard against selecting a driver whose package was never imported.

## Context propagation

`featureflag.ContextWithEvaluator(ctx, eval)` attaches an `*Evaluator` to a context; `featureflag.FromContext(ctx)` retrieves it (`ok == false` when absent). `featureflag.FromApp(app)` is the canonical way to retrieve the evaluator built by `featureflag.Module`.

## Lifecycle

`featureflag.Module` registers `Evaluator.Shutdown` as an `OnShutdown` callback, which closes the underlying provider. Only one `featureflag.Module` may be initialised per App — a second init returns `featureflag: Module already initialised`.
