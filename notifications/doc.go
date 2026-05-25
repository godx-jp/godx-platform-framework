// Package notifications implements Laravel-style notifications —
// route a Notifiable + Notification to mail, Slack, Discord, webhook,
// database, or log channels.
//
//	app := framework.New("svc", "1.0.0").
//	    Use(mail.Module).
//	    Use(notifications.Module)
//	mgr, _ := notifications.FromApp(app)
//	_ = mgr.Send(ctx, user, WelcomeNotification{})
//
// Channel matrix:
//
//	log      - slog (light, auto)
//	mail     - uses mail.Manager (requires mail.Module)
//	slack    - incoming webhook JSON (light, auto)
//	discord  - Discord webhook JSON (light, auto)
//	webhook  - generic HTTP POST (light, auto; uses httpclient when wired)
//	database - caller-provided DatabaseStore (configured in ModuleWithConfig)
package notifications
