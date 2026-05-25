# Notifications

> Laravel-style `Send(ctx, notifiable, notification)` routing to mail,
> Slack, Discord, webhook, database, or log channels.

## Concepts

A `Notifiable` routes addresses per channel (`RouteNotificationFor`).
A `Notification` declares channels via `Via` and optional presenter
interfaces (`MailPresenter`, `SlackPresenter`, …). The `*notifications.Manager`
delivers to each channel and emits lifecycle events when `events.Module`
is wired.

## Quick start

```go
import (
    "context"

    "github.com/godx-jp/godx-platform-framework/framework"
    "github.com/godx-jp/godx-platform-framework/mail"
    "github.com/godx-jp/godx-platform-framework/notifications"
)

app := framework.New("svc", "1.0.0").
    Use(mail.Module).
    Use(notifications.Module)
_ = app.Init(ctx)

mgr, _ := notifications.FromApp(app)
_ = mgr.Send(ctx, user, WelcomeNotification{})
```

## Channels

| Name       | Registration | Depends on |
|------------|--------------|------------|
| `log`      | auto         | — |
| `mail`     | module-built | `mail.Module` |
| `slack`    | auto         | webhook URL |
| `discord`  | auto         | webhook URL |
| `webhook`  | auto         | URL; uses `httpclient` when wired |
| `database` | module-built | `Config.DatabaseStore` |

## Presenter interfaces

Implement only the presenters for channels returned by `Via`:

```go
func (Welcome) Via(n Notifiable) []string { return []string{"mail", "log"} }
func (Welcome) ToMail(n Notifiable) MailMessage {
    return MailMessage{Subject: "Hi", Body: "Welcome"}
}
```

## Database channel

Provide a `driver.DatabaseStore` when configuring the module:

```go
cfg := notifications.Config{
    Default: "database",
    DatabaseStore: myRepo,
    Channels: map[string]notifications.ChannelConfig{
        "database": {Driver: "database"},
    },
}
app.Use(notifications.ModuleWithConfig(cfg))
```

## Environment variables

| Variable                   | Default | Notes |
|----------------------------|---------|-------|
| `NOTIFICATIONS_DEFAULT`    | `log`   | default channel name |
| `NOTIFICATIONS_CHANNELS`   | (default only) | CSV of channel names |
| `NOTIFICATIONS_WEBHOOK_URL`|         | default webhook for slack/discord/webhook |
| `NOTIFICATIONS_MAIL_MAILER`|         | mail transport name on mail.Manager |

## Events

| Event                   | When |
|-------------------------|------|
| `notification.sending`  | before channel Send |
| `notification.sent`     | after successful Send |
| `notification.failed`   | when Send returns an error |
