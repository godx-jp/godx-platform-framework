# Messaging

> Cross-service integration events as CloudEvents v1.0 over swappable
> brokers. Publish and subscribe through one Laravel-style facade; pick
> the broker per environment with configuration.

## Concepts

A `*messaging.Manager` owns named broker connections, each backed by a driver chosen at deploy time (`memory`, `nats`, `kafka`). `Publisher` and `Subscriber` wrap a broker and marshal payloads through the `envelope` package, so application code always speaks CloudEvents and never imports a driver.

```
Manager ── named connections
   ├─ Publisher(conn) ── envelope.Encode → driver.Broker.Publish
   └─ Subscriber(conn) ── driver.Broker.Subscribe → envelope.Decode
         └─ driver.Broker (memory · nats · kafka)
```

Three event layers in the framework, easily confused:

| Layer | Module | Purpose |
|-------|--------|---------|
| Domain events | `events` | In-process listeners |
| Integration events | `messaging` | Service ↔ service pub/sub |
| Background jobs | `queue` | Task workers |

## Quick start

```go
package main

import (
    "context"

    "github.com/godx-jp/godx-platform-framework/framework"
    "github.com/godx-jp/godx-platform-framework/messaging"
    "github.com/godx-jp/godx-platform-framework/messaging/envelope"
)

func main() {
    ctx := context.Background()
    app := framework.New("svc", "1.0.0").Use(messaging.Module)
    if err := app.Init(ctx); err != nil { panic(err) }
    defer app.Shutdown(ctx)

    mgr, _ := messaging.FromApp(app)
    pub, _ := mgr.Publisher()
    _ = pub.Publish(ctx, envelope.Event{
        ID:     "evt-1",
        Source: "orders",
        Type:   "orders.order.placed.v1",
        Data:   []byte(`{"order_id":"123"}`),
    })
}
```

With nothing in the environment you get one in-memory connection named `platform`.

## Configuration

### Environment variables

| Variable | Default | Notes |
|----------|---------|-------|
| `MESSAGING_DEFAULT` | `platform` | default connection name |
| `MESSAGING_CONNECTIONS` | (default only) | CSV of connection names |
| `MESSAGING_CONN_<NAME>_DRIVER` | `memory` | driver for the connection |
| `MESSAGING_CONN_<NAME>_SUBJECT_PREFIX` | — | prefix prepended to subjects |
| `MESSAGING_CONN_<NAME>_NATS_URL` | — | NATS server URL |
| `MESSAGING_CONN_<NAME>_JETSTREAM_STREAM` | — | JetStream stream name |
| `MESSAGING_CONN_<NAME>_KAFKA_TOPIC` | — | Kafka topic |
| `MESSAGING_CONN_<NAME>_KAFKA_BROKERS` | — | CSV of Kafka broker addresses |

`<NAME>` is the uppercased connection name. When `MESSAGING_CONNECTIONS` is empty, a single connection named after `MESSAGING_DEFAULT` is created.

### Programmatic config

```go
cfg := messaging.Config{
    Default: "platform",
    Connections: map[string]messaging.ConnConfig{
        "platform": {Driver: driver.Spec{Name: "memory"}},
    },
}
app := framework.New(...).Use(messaging.ModuleWithConfig(cfg))
```

`driver.Spec` carries `Name`, `SubjectPrefix`, `NATSURL`, `StreamName`, `KafkaBrokers`, `KafkaTopic`, and a free-form `Extra map[string]string`.

## API

| Type / method | Signature | Notes |
|---|---|---|
| `(*Manager).Publisher(conn ...string)` | `(*Publisher, error)` | Publisher bound to a connection (default when omitted) |
| `(*Manager).Subscriber(conn ...string)` | `(*Subscriber, error)` | Subscriber bound to a connection |
| `(*Manager).Add(name string, b driver.Broker)` | `error` | Register a broker; first registered becomes default |
| `(*Manager).SetDefault(name string)` | `error` | Choose the default connection |
| `(*Manager).Shutdown(ctx)` | `error` | Close every broker; returns the first error |
| `(*Publisher).Publish(ctx, e envelope.Event)` | `error` | Validates, encodes, and publishes; subject is `e.Subject` or falls back to `e.Type`; sets `ce-id` / `ce-type` headers |
| `(*Subscriber).Subscribe(ctx, subject string, fn func(ctx, envelope.Event) error)` | `(driver.Subscription, error)` | Decodes each message before invoking `fn` |
| `FromApp(app)` | `(*Manager, error)` | Retrieve the manager built by `messaging.Module` |

## CloudEvents envelope

`messaging/envelope` is the wire format — CloudEvents `specversion` `1.0`.

```go
type Event struct {
    ID              string
    Source          string
    Type            string
    Subject         string
    Time            time.Time
    DataContentType string
    Data            []byte
}
```

| Function | Notes |
|---|---|
| `(Event).Validate() error` | Requires `ID`, `Source`, and `Type` |
| `Encode(e Event) ([]byte, error)` | Defaults `Time` to now (UTC) and `DataContentType` to `application/json`; `Data` is embedded as raw JSON |
| `Decode(b []byte) (Event, error)` | Parses the JSON envelope and validates the result |

## Domain → integration bridge

`WireBridge` listens on an in-process `events.Bus` and republishes matching events as CloudEvents through a `Publisher`:

```go
bridge, _ := messaging.WireBridge(ctx, bus, pub, messaging.BridgeOptions{
    Prefix: "integration.", // only event names with this prefix (empty = all)
    Source: "godx://orders",
})
defer bridge.Close()
```

`BridgeOptions` fields: `Prefix` (name filter), `Source` (CloudEvents source URI, defaults to `godx://local`), and `TypePrefix` (prepended to the event name for `ce-type`, defaults to `com.godx.`). The bridge subscribes with pattern `<Prefix>*` (or `*`), derives an `ID` from the event name plus `CreatedAt`, and serializes the payload to bytes (`[]byte`/`string` pass through, everything else via `json.Marshal`). `Close()` cancels the bus subscription.

`ForwardListener(pub *Publisher, source string) events.Listener` is a lower-level alternative: a plain `events.Listener` that forwards each event, taking its `ID` from the event metadata `id` (or synthesizing one). Unlike `WireBridge` it does not apply a `TypePrefix` or name filter.

## Transactional outbox

`messaging/outbox` relays rows from a database outbox table to the broker, so a service can write domain changes and the event to publish in one transaction, then publish asynchronously.

```go
type Store interface {
    FetchUnpublished(ctx context.Context, limit int) ([]Row, error)
    MarkPublished(ctx context.Context, ids []string) error
}

type Row struct {
    ID        string
    EventType string
    Payload   []byte
    CreatedAt time.Time
}
```

`RunRelay(ctx, store Store, pub *messaging.Publisher, opts RelayOptions) error` fetches one batch (`RelayOptions.BatchSize`, default `100`), publishes each row as a CloudEvent (`ID` = row ID, `Type` = `EventType`, `Source` = `opts.Source`, `Data` = `Payload`), then marks the published IDs. It is a single pass — the application schedules it on a ticker / worker loop. A nil `store` or `pub` is a no-op.

## Driver matrix

| Driver | Status | Registration | Notes |
|---|---|---|---|
| `memory` | stable | auto | In-process broker. Light. Ideal for tests and single-process services |
| `nats` | stable | opt-in (`_ "...messaging/drivers/nats"`) | NATS / JetStream. Heavy |
| `kafka` | stub | opt-in (`_ "...messaging/drivers/kafka"`) | Registered but its constructor returns `driver.ErrNotImplemented` — not usable yet |

The `memory` driver auto-registers on `import "...messaging"`. Heavy drivers register only on an explicit blank import:

```go
import _ "github.com/godx-jp/godx-platform-framework/messaging/drivers/nats"
```

Selecting a driver whose package was not imported fails at module init with `messaging: driver %q not registered`.

## Error model

```go
import mdriver "github.com/godx-jp/godx-platform-framework/messaging/driver"

mdriver.ErrClosed         // broker used after Close
mdriver.ErrNotImplemented // returned by the kafka stub constructor
```

`Publish` returns the envelope `Validate` error when `ID`, `Source`, or `Type` is missing, before touching the broker. `driver.New` returns a "driver %q not registered" error for an unknown / unimported driver name.

## Lifecycle

`messaging.Module` publishes the `*Manager` on the framework `App` under store key `godx.messaging.manager` and registers an `OnShutdown` callback that calls `Manager.Shutdown`, closing every broker connection. Retrieve the manager with `messaging.FromApp(app)`.
