// Run with `go run ./examples/notifications` from the repo root.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/godx-jp/godx-platform-framework/framework"
	mdriver "github.com/godx-jp/godx-platform-framework/mail/driver"
	"github.com/godx-jp/godx-platform-framework/mail"
	"github.com/godx-jp/godx-platform-framework/notifications"
	ndriver "github.com/godx-jp/godx-platform-framework/notifications/driver"
)

type user struct {
	email string
}

func (u user) RouteNotificationFor(channel string) string {
	if channel == "mail" || channel == ndriver.DriverMail {
		return u.email
	}
	return ""
}

type welcome struct{}

func (welcome) Via(notifications.Notifiable) []string {
	return []string{"log", "mail"}
}

func (welcome) ToMail(notifications.Notifiable) notifications.MailMessage {
	return notifications.MailMessage{
		Subject: "Welcome",
		Body:    "Thanks for joining.",
	}
}

func main() {
	ctx := context.Background()
	mailCfg := mail.Config{
		Default: "primary",
		From:    "noreply@example.com",
		Mailers: map[string]mail.MailerConfig{
			"primary": {Driver: mdriver.DriverLog, Spec: mdriver.Spec{Name: mdriver.DriverLog}},
		},
	}
	notifyCfg := notifications.Config{
		Default: "log",
		Channels: map[string]notifications.ChannelConfig{
			"log":  {Driver: ndriver.DriverLog, Spec: ndriver.Spec{Name: ndriver.DriverLog}},
			"mail": {Driver: ndriver.DriverMail, Spec: ndriver.Spec{Name: ndriver.DriverMail}},
		},
	}
	app := framework.New("notifications-example", "0.0.0").
		Use(mail.ModuleWithConfig(mailCfg)).
		Use(notifications.ModuleWithConfig(notifyCfg))
	if err := app.Init(ctx); err != nil {
		log.Fatalf("init: %v", err)
	}
	defer func() { _ = app.Shutdown(ctx) }()

	mgr, err := notifications.FromApp(app)
	if err != nil {
		log.Fatalf("notifications: %v", err)
	}
	if err := mgr.Send(ctx, user{email: "user@example.com"}, welcome{}); err != nil {
		log.Fatalf("send: %v", err)
	}
	fmt.Println("notification sent via log + mail channels")
}
