# Health

> Kubernetes-style `/healthz` (liveness) and `/readyz` (readiness) with dependency probes.

## Quick start

```go
app := framework.New("svc", "1.0.0").Use(health.Module)
_ = app.Init(ctx)

reg, _ := health.FromApp(app)
reg.RegisterProbe("database", db.Ping)

http.ListenAndServe(":8080", health.Handler(reg, health.Options{}))
```

## Endpoints

| Path | Purpose |
|------|---------|
| `/healthz` | Liveness — returns 200 when process is up |
| `/readyz` | Readiness — runs all `RegisterProbe` checks; 503 when any fail |

## chi integration

```go
health.Mount(r, reg, health.Options{ProbeTimeout: 3 * time.Second})
```

See [examples/health/main.go](../../examples/health/main.go).
