# Mail

> Fluent `To` / `Subject` / `Body` / `Send` mailer backed by log, SMTP,
> SES, SendGrid, Mailgun, or Postmark. Emits lifecycle events when
> `events.Module` is wired.

## Concepts

The mail module is a thin facade over six transports. A `*mail.Manager`
owns named mailers; `Mailer()` returns a fluent builder. When an
`events.Bus` is available (from `events.Module` or context), `Send`
dispatches `mail.sending`, `mail.sent`, and `mail.failed`.

## Quick start

```go
import (
    "context"

    "github.com/godx-jp/godx-platform-framework/framework"
    "github.com/godx-jp/godx-platform-framework/mail"
)

ctx := context.Background()
app := framework.New("svc", "1.0.0").Use(mail.Module)
if err := app.Init(ctx); err != nil { /* … */ }
defer app.Shutdown(ctx)

mgr, _ := mail.FromApp(app)
_ = mgr.Mailer().To("user@example.com").Subject("Hi").Body("Hello").Send(ctx)
```

## Drivers

| Name       | Visibility   | Notes |
|------------|--------------|-------|
| `log`      | auto         | writes to slog — default zero-config driver |
| `smtp`     | auto         | `net/smtp` with optional plain auth |
| `ses`      | blank-import | Amazon SES v2 (AWS SDK) |
| `sendgrid` | blank-import | SendGrid v3 REST API |
| `mailgun`  | blank-import | Mailgun REST API |
| `postmark` | blank-import | Postmark REST API |

Heavy drivers require a blank import:

```go
import (
    _ "github.com/godx-jp/godx-platform-framework/mail/drivers/ses"
    _ "github.com/godx-jp/godx-platform-framework/mail/drivers/sendgrid"
)
```

## Environment variables

| Variable              | Default | Notes |
|-----------------------|---------|-------|
| `MAIL_DEFAULT`        | `log`   | default mailer name |
| `MAIL_MAILERS`        | (default only) | CSV of mailer names |
| `MAIL_FROM`           |         | default From address |
| `MAIL_SMTP_HOST`      |         | SMTP host |
| `MAIL_SMTP_PORT`      | `587`   | SMTP port |
| `MAIL_SMTP_USERNAME`  |         | SMTP auth user |
| `MAIL_SMTP_PASSWORD`  |         | SMTP auth password |
| `MAIL_SES_REGION`     |         | AWS region override |
| `MAIL_API_KEY`        |         | SendGrid / Postmark / Mailgun API key |
| `MAIL_DOMAIN`         |         | Mailgun domain |

## Events

| Event          | When |
|----------------|------|
| `mail.sending` | before transport Send |
| `mail.sent`    | after successful Send |
| `mail.failed`  | when Send returns an error |

Payload includes `driver`, `from`, `to`, `subject`, and `error` (on failure).

## Laravel parity

Maps to Laravel's `Mail` facade and `Mailer` transports. Use `log` in
dev (like `log` mail driver) and swap to `smtp` / cloud providers in
production via env only.
