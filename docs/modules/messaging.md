# Messaging

> Cross-service integration events with CloudEvents v1.0 and swappable brokers.

## Concepts

| Layer | Module | Purpose |
|-------|--------|---------|
| Domain events | `events` | In-process listeners |
| Integration events | `messaging` | Service ↔ service pub/sub |
| Background jobs | `queue` | Task workers |

## Quick start

```go
app := framework.New("svc", "1.0.0").Use(messaging.Module)
_ = app.Init(ctx)
mgr, _ := messaging.FromApp(app)
pub, _ := mgr.Publisher()
_ = pub.Publish(ctx, envelope.Event{
    ID: "evt-1", Source: "orders", Type: "orders.order.placed.v1",
    Data: []byte(`{"order_id":"123"}`),
})
```

## Drivers

| Driver | Status |
|--------|--------|
| `memory` | stable (auto-register) |
| `nats` | stub (JetStream planned) |
| `kafka` | stub |

## Env vars

| Variable | Default |
|----------|---------|
| `MESSAGING_DEFAULT` | `platform` |
| `MESSAGING_CONNECTIONS` | default name |
| `MESSAGING_CONN_<NAME>_DRIVER` | `memory` |

See [messaging module plan](../../CHANGELOG.md) for outbox + NATS JetStream roadmap.
