# Notifications

> Laravel-style `Send(ctx, notifiable, notification)` routing to mail,
> Slack, Discord, webhook, database, or log channels. A notification
> declares its channels; presenters render the per-channel payload.

## Concepts

A `Notifiable` routes addresses per channel (`RouteNotificationFor`). A `Notification` declares its channels via `Via` and implements one presenter interface per channel (`MailPresenter`, `SlackPresenter`, …). The `*notifications.Manager` resolves each named channel, delivers the notification, and emits lifecycle events when `events.Module` is wired.

```
Manager ── named channels
   └─ Notifier.Send(notifiable, notification)
         └─ for each name in notification.Via(notifiable):
              driver.Channel.Send  (log · mail · slack · discord · webhook · database)
```

## Quick start

```go
package main

import (
    "context"

    "github.com/godx-jp/godx-platform-framework/framework"
    "github.com/godx-jp/godx-platform-framework/mail"
    "github.com/godx-jp/godx-platform-framework/notifications"
)

func main() {
    ctx := context.Background()
    app := framework.New("svc", "1.0.0").
        Use(mail.Module).
        Use(notifications.Module)
    if err := app.Init(ctx); err != nil { panic(err) }
    defer app.Shutdown(ctx)

    mgr, _ := notifications.FromApp(app)
    _ = mgr.Send(ctx, user, WelcomeNotification{})
}
```

With nothing in the environment you get a single `log` channel.

## Defining a notification

```go
type Welcome struct{}

func (Welcome) Via(n notifications.Notifiable) []string { return []string{"mail", "log"} }

func (Welcome) ToMail(n notifications.Notifiable) notifications.MailMessage {
    return notifications.MailMessage{Subject: "Hi", Body: "Welcome"}
}
```

Implement only the presenter for each channel `Via` returns. The root package re-exports the `contract` interfaces and message structs as type aliases so callers import one package.

| Presenter interface | Renders | Message struct |
|---|---|---|
| `MailPresenter.ToMail` | email | `MailMessage{To, Subject, Body, HTML}` |
| `SlackPresenter.ToSlack` | Slack incoming webhook | `SlackMessage{WebhookURL, Text}` |
| `DiscordPresenter.ToDiscord` | Discord webhook | `DiscordMessage{WebhookURL, Content}` |
| `WebhookPresenter.ToWebhook` | generic webhook POST | `WebhookMessage{URL, Body, Headers}` |
| `DatabasePresenter.ToDatabase` | persisted row | `DatabaseMessage{Type, Data}` |

`IdentifiableNotifiable` extends `Notifiable` with `NotifiableType()` / `NotifiableID()` — required by the database channel to record ownership.

## Configuration

### Environment variables

| Variable | Default | Notes |
|----------------------------|---------|-------|
| `NOTIFICATIONS_DEFAULT` | `log` | default channel name |
| `NOTIFICATIONS_CHANNELS` | (default only) | CSV of channel names |
| `NOTIFICATIONS_WEBHOOK_URL` | — | default webhook URL for slack / discord / webhook |
| `NOTIFICATIONS_MAIL_MAILER` | — | mail transport name on `mail.Manager` for the mail channel |

The channel name doubles as the driver name when loading from env.

### Programmatic config

The database channel cannot be configured from env — it needs a `driver.DatabaseStore`, so wire it through `ModuleWithConfig`:

```go
cfg := notifications.Config{
    Default:       "database",
    DatabaseStore: myRepo, // implements driver.DatabaseStore
    Channels: map[string]notifications.ChannelConfig{
        "database": {Driver: "database"},
    },
}
app.Use(notifications.ModuleWithConfig(cfg))
```

`Config.Validate` requires a non-empty `Default` present in `Channels`, at least one channel, a `Driver` per channel, and a `DatabaseStore` whenever any channel uses the `database` driver.

## Channel matrix

| Channel | Status | Registration | Notes |
|---|---|---|---|
| `log` | stable | auto | Writes the notification to slog. Light |
| `slack` | stable | auto | Posts to a Slack incoming webhook; uses `httpclient` when wired |
| `discord` | stable | auto | Posts to a Discord webhook; uses `httpclient` when wired |
| `webhook` | stable | auto | Generic JSON/text POST; uses `httpclient` when wired |
| `mail` | stable | module-built | Built by `notifications.Module` when `mail.Module` is present |
| `database` | stable | module-built | Built when `Config.DatabaseStore` is supplied |

The `log`, `slack`, `discord`, and `webhook` drivers auto-register on `import "...notifications"`. The `mail` and `database` channels are not registry drivers — the module constructs them directly from its dependencies (a `*mail.Manager` and a `DatabaseStore`). When the module finds an `httpclient` default client on the `App`, it injects it into the slack / discord / webhook channels via `SetHTTPClient`. Selecting an unregistered driver name fails at init with a message naming the missing import path (`…/notifications/drivers/<name>`).

## Manager API

| Method | Notes |
|---|---|
| `(*Manager).Send(ctx, notifiable, notification)` | Shorthand for `Notifier().Send(...)` |
| `(*Manager).Notifier()` | Returns a `*Notifier` bound to the manager |
| `(*Manager).Channel(name string)` | Look up a registered channel |
| `(*Manager).Names()` | Sorted channel names |
| `(*Manager).AddChannel(name, ch)` | Register a channel (first becomes default) |
| `(*Manager).SetDefault(name string)` | Choose the default channel |
| `(*Manager).SetBus(bus)` | Attach an events bus |
| `(*Manager).Shutdown(ctx)` | Shut down every channel |
| `(*Notifier).Send(ctx, notifiable, notification)` | Delivers to every channel from `Via` |

## Events

| Event | When |
|-------------------------|------|
| `notification.sending` | before each channel `Send` |
| `notification.sent` | after a successful channel `Send` |
| `notification.failed` | when a channel `Send` returns an error |

Payload includes `channel`, `driver`, and `error` (on failure). Events fire only when a bus is attached via `SetBus` or found on the context.

## Error model

`Notifier.Send` returns `nil` only when a notification declares at least one channel **and** every channel succeeds. A notification whose `Via` returns no channels is an error. Per-channel failures (unknown channel name, or a channel's own `Send` error) are collected and combined with `errors.Join`, so delivery to the remaining channels still proceeds. A channel returns `driver.ErrClosed` after `Shutdown`.

## Context propagation

`notifications.ContextWithManager(ctx, mgr)` attaches a manager to a context; `notifications.FromContext(ctx)` retrieves it. The manager also looks up an `events.Bus` from the context when it has none. `notifications.FromApp(app)` is the canonical way to retrieve the manager built by `notifications.Module`.

## Lifecycle

`notifications.Module` publishes the `*Manager` on the framework `App` under store key `godx.notifications.manager` and registers an `OnShutdown` callback that calls `Manager.Shutdown`, shutting down every channel. Retrieve the manager with `notifications.FromApp(app)`.

## Security: SSRF protection

The `webhook`, `slack` and `discord` channels post to a destination URL that may be supplied at runtime (per-notification message, or the notifiable's `RouteNotificationFor`). Because that URL is attacker-influenced, every destination is validated against an SSRF allow/deny policy **before** any request is issued:

- Only `http` and `https` schemes are permitted (`http` is allowed for on-prem Slack/webhook endpoints). Everything else — `file:`, `gopher:`, `ftp:`, … — is rejected.
- The host is checked as a literal IP, and otherwise resolved via DNS; the request is blocked if any resolved address falls in a private, loopback, link-local, unique-local or unspecified range. This includes `127.0.0.0/8`, `10/8`, `172.16/12`, `192.168/16`, `169.254/16` (the AWS/GCP **metadata IP** `169.254.169.254`), `::1`, `fc00::/7`, `fe80::/10`, `0.0.0.0`, and IPv4-mapped equivalents.

A blocked destination returns an error wrapping `ErrBlockedURL` and **no HTTP request is sent**. The injected HTTP client does not follow redirects for these channels; if redirect-following is ever enabled, the redirect target must be re-validated, since a `30x` to an internal host would otherwise bypass the check.
