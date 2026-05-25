// Package contract holds notification interfaces shared by the root
// notifications package and channel drivers (avoids import cycles).
package contract

// Notifiable routes outbound notifications per channel.
type Notifiable interface {
	RouteNotificationFor(channel string) string
}

// Notification selects delivery channels.
type Notification interface {
	Via(notifiable Notifiable) []string
}

// MailPresenter renders an email notification.
type MailPresenter interface {
	ToMail(notifiable Notifiable) MailMessage
}

// SlackPresenter renders a Slack incoming-webhook payload.
type SlackPresenter interface {
	ToSlack(notifiable Notifiable) SlackMessage
}

// DiscordPresenter renders a Discord webhook payload.
type DiscordPresenter interface {
	ToDiscord(notifiable Notifiable) DiscordMessage
}

// WebhookPresenter renders a generic webhook POST body.
type WebhookPresenter interface {
	ToWebhook(notifiable Notifiable) WebhookMessage
}

// DatabasePresenter renders a row for the database channel.
type DatabasePresenter interface {
	ToDatabase(notifiable Notifiable) DatabaseMessage
}

// IdentifiableNotifiable adds type/id metadata for database storage.
type IdentifiableNotifiable interface {
	Notifiable
	NotifiableType() string
	NotifiableID() string
}

// MailMessage is sent via the mail channel.
type MailMessage struct {
	To      []string
	Subject string
	Body    string
	HTML    string
}

// SlackMessage posts to a Slack incoming webhook.
type SlackMessage struct {
	WebhookURL string
	Text       string
}

// DiscordMessage posts to a Discord webhook.
type DiscordMessage struct {
	WebhookURL string
	Content    string
}

// WebhookMessage is a generic JSON/text webhook POST.
type WebhookMessage struct {
	URL     string
	Body    []byte
	Headers map[string]string
}

// DatabaseMessage is persisted by the database channel.
type DatabaseMessage struct {
	Type string
	Data []byte
}
