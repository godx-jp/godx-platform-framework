package notifications

import "github.com/godx-jp/godx-platform-framework/notifications/contract"

type (
	Notifiable               = contract.Notifiable
	Notification             = contract.Notification
	MailPresenter            = contract.MailPresenter
	SlackPresenter           = contract.SlackPresenter
	DiscordPresenter         = contract.DiscordPresenter
	WebhookPresenter         = contract.WebhookPresenter
	DatabasePresenter        = contract.DatabasePresenter
	IdentifiableNotifiable   = contract.IdentifiableNotifiable
	MailMessage              = contract.MailMessage
	SlackMessage             = contract.SlackMessage
	DiscordMessage           = contract.DiscordMessage
	WebhookMessage           = contract.WebhookMessage
	DatabaseMessage          = contract.DatabaseMessage
)
