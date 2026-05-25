# Mail

> Fluent `To` / `Subject` / `Body` / `Send` mailer backed by log, SMTP,
> SES, SendGrid, Mailgun, or Postmark. Pick a transport per environment
> via configuration; emits lifecycle events when `events.Module` is wired.

## Concepts

A `*mail.Manager` owns named transports; `Mailer()` returns a fluent builder bound to one of them. Each transport is a driver chosen at deploy time — application code never imports a driver. When an `events.Bus` is available (from `events.Module` or context), `Send` dispatches `mail.sending`, `mail.sent`, and `mail.failed`.

```
Manager ── named transports + default From
   └─ Mailer() ── To / From / Subject / Body / HTML / Send
         └─ driver.Transport (log · smtp · ses · sendgrid · mailgun · postmark)
```

## Quick start

```go
package main

import (
    "context"

    "github.com/godx-jp/godx-platform-framework/framework"
    "github.com/godx-jp/godx-platform-framework/mail"
)

func main() {
    ctx := context.Background()
    app := framework.New("svc", "1.0.0").Use(mail.Module)
    if err := app.Init(ctx); err != nil { panic(err) }
    defer app.Shutdown(ctx)

    mgr, _ := mail.FromApp(app)
    _ = mgr.Mailer().
        To("user@example.com").
        Subject("Hi").
        Body("Hello").
        Send(ctx)
}
```

With nothing in the environment you get a single `log` transport that writes to slog.

## Configuration

### Environment variables

| Variable | Default | Notes |
|----------|---------|-------|
| `MAIL_DEFAULT` | `log` | default mailer name |
| `MAIL_MAILERS` | (default only) | CSV of mailer names |
| `MAIL_FROM` | — | default From address applied when a `Mailer` does not override it |
| `MAIL_SMTP_HOST` | — | SMTP host |
| `MAIL_SMTP_PORT` | `587` | SMTP port — the env loader leaves it `0`; the smtp driver falls back to `587` |
| `MAIL_SMTP_USERNAME` | — | SMTP auth user |
| `MAIL_SMTP_PASSWORD` | — | SMTP auth password |
| `MAIL_SES_REGION` | — | AWS region for the SES driver |
| `MAIL_API_KEY` | — | SendGrid / Postmark / Mailgun API key |
| `MAIL_DOMAIN` | — | Mailgun domain |

The mailer name doubles as the driver name when loading from env — `MAIL_MAILERS=smtp` builds a transport whose driver is `smtp`. The matching env keys are read by driver: SMTP reads the `MAIL_SMTP_*` keys, SES reads `MAIL_SES_REGION`, SendGrid/Postmark read `MAIL_API_KEY`, Mailgun reads `MAIL_API_KEY` + `MAIL_DOMAIN`.

### Programmatic config

```go
import mdriver "github.com/godx-jp/godx-platform-framework/mail/driver"

cfg := mail.Config{
    Default: "smtp",
    From:    "noreply@example.com",
    Mailers: map[string]mail.MailerConfig{
        "smtp": {Driver: "smtp", Spec: mdriver.Spec{Host: "localhost", Port: 1025}},
        "log":  {Driver: "log"},
    },
}
app := framework.New(...).Use(mail.ModuleWithConfig(cfg))
```

`Config.Validate` requires a non-empty `Default` that is present in `Mailers`, at least one mailer, and a non-empty `Driver` per mailer. `Spec` is the uniform driver input (`Name`, `From`, SMTP `Host`/`Port`/`Username`/`Password`/`TLS`, `Region`, `APIKey`, `Domain`, and free-form `Extra`).

## Manager & Mailer API

| Method | Notes |
|---|---|
| `(*Manager).Mailer(name ...string)` | Fluent builder for the default (or named) transport |
| `(*Manager).Transport(name string)` | Look up a transport by name |
| `(*Manager).DefaultTransport()` | The default transport, or nil |
| `(*Manager).Names()` | Sorted transport names |
| `(*Manager).AddTransport(name, t)` | Register a transport (first becomes default) |
| `(*Manager).SetDefault(name string)` | Choose the default transport |
| `(*Manager).SetBus(bus)` / `SetDefaultFrom(from)` | Attach an events bus / fallback From |
| `(*Manager).Shutdown(ctx)` | Shut down every transport |

The `Mailer` builder is chainable: `To(addrs...)`, `From(addr)`, `Subject(s)`, `Body(b)`, `HTML(h)`, then `Send(ctx)`. `Send` requires at least one recipient and a bound transport, falling back to the manager's default From when none is set.

## Driver matrix

| Driver | Status | Registration | Notes |
|---|---|---|---|
| `log` | stable | auto | Writes the message to slog. Light. Default zero-config driver |
| `smtp` | stable | auto | `net/smtp` with optional plain auth; default port `587`. Light |
| `ses` | stable | opt-in (`_ "...mail/drivers/ses"`) | Amazon SES (AWS SDK). Heavy |
| `sendgrid` | stable | opt-in (`_ "...mail/drivers/sendgrid"`) | SendGrid v3 REST API. Heavy |
| `mailgun` | stable | opt-in (`_ "...mail/drivers/mailgun"`) | Mailgun REST API. Heavy |
| `postmark` | stable | opt-in (`_ "...mail/drivers/postmark"`) | Postmark REST API. Heavy |

**Light** drivers (`log`, `smtp`) auto-register on `import "...mail"`.

**Heavy** drivers carry an SDK / HTTP client and register only on an explicit blank import, so binaries that only log mail stay free of the dependency:

```go
import (
    _ "github.com/godx-jp/godx-platform-framework/mail/drivers/ses"
    _ "github.com/godx-jp/godx-platform-framework/mail/drivers/sendgrid"
)
```

Selecting a driver whose package was not imported fails at module init with a message naming the missing import path (`…/mail/drivers/<name>`).

## Events

| Event | When |
|----------------|------|
| `mail.sending` | before transport `Send` |
| `mail.sent` | after successful `Send` |
| `mail.failed` | when `Send` returns an error |

Payload includes `driver`, `from`, `to`, `subject`, and `error` (on failure). Events fire only when a bus is attached via `SetBus` or found on the context.

## Error model

```go
import mdriver "github.com/godx-jp/godx-platform-framework/mail/driver"

// A transport returns ErrClosed after Shutdown.
err := mgr.Mailer().To("x@y").Subject("s").Body("b").Send(ctx)
if errors.Is(err, mdriver.ErrClosed) { /* transport already shut down */ }
```

`Send` also returns plain errors for a missing transport or empty recipient list; transport-specific failures (SMTP, REST) propagate from the driver and trigger `mail.failed`.

## Security

The `smtp` driver guards against SMTP header / CRLF injection before it ever dials the server:

- Every address — the `From` and each `To` — is parsed with `net/mail.ParseAddress` and rejected if it fails to parse or contains a CR (`\r`) or LF (`\n`). Both bare addresses (`a@b.com`) and display-name form (`Name <a@b.com>`) remain valid.
- The `Subject` is rejected if it contains CR, LF, or any other ASCII control character, and is additionally Q-encoded (`mime.QEncoding`) so non-ASCII and edge-case bytes cannot break out into a new header line.

A message that fails validation never reaches the wire; `Send` returns a descriptive error (wrapping the sentinel `smtp.ErrHeaderInjection` for control-character cases) instead of smuggling extra headers such as `Bcc:` or a forged `Content-Type`.

The `log` transport **does not emit the full message body by default**. It logs the recipient, subject, body length (`body_bytes`), and a short truncated `body_preview`, so secrets in the body — reset links, tokens, OTPs — are not leaked to logs if the transport is ever selected outside dev. Full-body logging is available only by explicitly constructing the transport with `log.NewWithBody`, and should be used only in trusted dev environments.

## Context propagation

`mail.ContextWithManager(ctx, mgr)` attaches a manager to a context; `mail.FromContext(ctx)` retrieves it. `Send` also looks up an `events.Bus` from the context when the manager has none. `mail.FromApp(app)` is the canonical way to retrieve the manager built by `mail.Module`.

## Lifecycle

`mail.Module` publishes the `*Manager` on the framework `App` under store key `godx.mail.manager` and registers an `OnShutdown` callback that calls `Manager.Shutdown`, shutting down every transport. Retrieve the manager with `mail.FromApp(app)`.

## Laravel parity

Maps to Laravel's `Mail` facade and mailer transports. Use `log` in dev and swap to `smtp` or a cloud provider in production via env only.
