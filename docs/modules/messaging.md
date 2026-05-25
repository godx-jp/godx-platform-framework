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

## Domain → integration bridge

Forward in-process domain events to the integration broker:

```go
bus := // from events.Module
pub, _ := mgr.Publisher()
bridge, _ := messaging.WireBridge(ctx, bus, pub, messaging.BridgeOptions{
    Prefix: "integration.",
    Source: "godx://orders",
})
defer bridge.Close()
```

## Drivers

| Driver | Status | Registration |
|--------|--------|--------------|
| `memory` | stable | auto |
| `nats` | stable (JetStream) | opt-in blank import |
| `kafka` | stub | opt-in |

```go
import _ "github.com/godx-jp/godx-platform-framework/messaging/drivers/nats"
```

## Env vars

| Variable | Default |
|----------|---------|
| `MESSAGING_DEFAULT` | `platform` |
| `MESSAGING_CONNECTIONS` | default name |
| `MESSAGING_CONN_<NAME>_DRIVER` | `memory` |
| `MESSAGING_CONN_<NAME>_NATS_URL` | — |
| `MESSAGING_CONN_<NAME>_STREAM` | optional JetStream stream |

Transactional outbox relay lives in `messaging/outbox` (application wires DB + relay worker).
