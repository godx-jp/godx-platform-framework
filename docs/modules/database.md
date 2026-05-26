# Database

> Laravel-faithful connection manager — not an ORM. Services keep sqlc,
> goose migrations, and domain repositories; this module owns pool
> lifecycle, read/write routing, query observability, and health probes.

## Concepts

```
Manager ── named Connections (postgres · mysql · sqlite)
   ├─ Write() ── primary connection
   ├─ Read(ctx) ── replica selection + sticky-after-write
   └─ WithTx ── serialization/deadlock retry helper
```

TBK tenancy GUCs (`SET LOCAL app.*`) live in `go-common/tenancy`, not here.
Transactional outbox stores live in `go-common/messaging/postgres`.

## Quick start (framework module)

```go
package main

import (
    "context"

    "github.com/godx-jp/godx-platform-framework/database"
    "github.com/godx-jp/godx-platform-framework/framework"

    _ "github.com/godx-jp/godx-platform-framework/database/drivers/postgres"
)

func main() {
    ctx := context.Background()
    app := framework.New("svc", "1.0.0").Use(database.Module)
    if err := app.Init(ctx); err != nil { panic(err) }
    defer app.Shutdown(ctx)

    mgr, _ := database.FromApp(app)
    write, _ := mgr.Write()
    pool := write.Postgres()
    _ = pool
}
```

## TBK services — platform.Boot

```go
rt, err := platform.Boot(ctx, platform.Config{
    ServiceName: "order-service",
    Database: platform.DatabaseConfig{
        URL:    cfg.DatabaseURL,
        Enable: true,
    },
})
pool, err := rt.Pool(ctx)
```

## Environment

| Variable | Default | Description |
|----------|---------|-------------|
| `DATABASE_URL` | — | Fallback URL for default connection |
| `DATABASE_DEFAULT_CONNECTION` | `default` | Default connection name |
| `DATABASE_CONNECTIONS` | `default` | Comma-separated connection names |
| `DATABASE_CONNECTION_<NAME>_DRIVER` | `postgres` | Driver for named connection |
| `DATABASE_CONNECTION_<NAME>_URL` | — | DSN for named connection |
| `DATABASE_WRITE_CONNECTION` | default | Write connection name |
| `DATABASE_READ_CONNECTIONS` | — | Comma-separated read replicas |
| `DATABASE_LOG_QUERIES` | `false` | Log SQL via pgx tracer |
| `DATABASE_TRACE_QUERIES` | `false` | OpenTelemetry spans per query |
| `DATABASE_METRICS_ENABLED` | `true` | Pool stats via OTel meter |

## Transactions

```go
import godxdb "github.com/godx-jp/godx-platform-framework/database"

err := godxdb.WithTx(ctx, pool, godxdb.TxOptions{}, func(tx pgx.Tx) error {
    _, err := tx.Exec(ctx, "INSERT INTO foo VALUES ($1)", id)
    return err
})
```

## Notifications store (v1.3)

`database/notifications.PostgresStore` implements `notifications/driver.DatabaseStore`
for the in-app notifications channel:

```go
import dbnotif "github.com/godx-jp/godx-platform-framework/database/notifications"

store, err := dbnotif.FromManager(mgr, dbnotif.LoadStoreConfigFromEnv())
```

Reference DDL for the `notifications` table belongs in service migrations.

## Drivers

| Driver | Import |
|--------|--------|
| Postgres (pgx) | `_ "github.com/godx-jp/godx-platform-framework/database/drivers/postgres"` |
| MySQL | `_ "github.com/godx-jp/godx-platform-framework/database/drivers/mysql"` |
| SQLite | `_ "github.com/godx-jp/godx-platform-framework/database/drivers/sqlite"` |

## Related

- [health](./health.md) — auto `database:write:*` / `database:read:*` probes
- [notifications](./notifications.md) — database channel wiring
- TBK `go-common/tenancy` — RLS GUC session variables
- TBK `go-common/messaging/postgres` — outbox + eventbus stores
