// Package mail implements a Laravel-style mailer — a fluent To /
// Subject / Body / Send facade backed by log, SMTP, SES, SendGrid,
// Mailgun, or Postmark transports.
//
//	app := framework.New("svc", "1.0.0").Use(mail.Module)
//	if err := app.Init(ctx); err != nil { return err }
//	mgr, _ := mail.FromApp(app)
//	_ = mgr.Mailer().To("user@example.com").Subject("Hi").Body("Hello").Send(ctx)
//
// When events.Module is also wired, Send dispatches mail.sending,
// mail.sent, and mail.failed on the shared Bus.
//
// Driver matrix:
//
//	log       - writes to slog (light, auto)
//	smtp      - net/smtp (light, auto)
//	ses       - Amazon SES v2 (heavy, opt-in)
//	sendgrid  - SendGrid REST API (heavy, opt-in)
//	mailgun   - Mailgun REST API (heavy, opt-in)
//	postmark  - Postmark REST API (heavy, opt-in)
package mail
