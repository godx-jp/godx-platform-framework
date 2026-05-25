# Feature flags

> Pluggable feature evaluation with optional in-memory caching.

## Quick start

```go
app := framework.New("svc", "1.0.0").
    Use(config.Module).
    Use(featureflag.Module)
_ = app.Init(ctx)
eval, _ := featureflag.FromApp(app)

if ok, _ := eval.Enabled(ctx, "new-checkout", userID, attrs); ok {
    // new path
}
```

## Drivers

| Driver | Registration | Notes |
|--------|--------------|-------|
| `config` | auto | Reads `flags.<name>` bool and optional `flags.<name>.users` allowlist |
| `openfeature` | opt-in | Heavy stub — requires `FEATUREFLAG_ENDPOINT` |
| `launchdarkly` | opt-in | Heavy stub — requires `FEATUREFLAG_SDK_KEY` |
| `unleash` | opt-in | Heavy stub — requires `FEATUREFLAG_ENDPOINT` |
| `flagsmith` | opt-in | Heavy stub — requires `FEATUREFLAG_SDK_KEY` |

## Config driver keys

```yaml
flags:
  new-checkout: true
  beta-ui:
    users:
      - alice@example.com
      - bob@example.com
```

## Env vars

| Variable | Default | Purpose |
|----------|---------|---------|
| `FEATUREFLAG_DRIVER` | `config` | Provider driver |
| `FEATUREFLAG_PREFIX` | `flags` | Config key prefix |
| `FEATUREFLAG_CACHE` | `false` | Enable per-eval in-memory cache |
| `FEATUREFLAG_CACHE_TTL` | `1m` | Cache entry TTL |

## API

```go
eval.Enabled(ctx, flag, user, attrs map[string]any) (bool, error)
```

`user` and `attrs` are forwarded to heavy drivers when fully implemented.
