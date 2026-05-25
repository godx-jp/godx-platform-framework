package driver

// Spec is the uniform input to notification channel constructors.
type Spec struct {
	// Name is the channel driver name.
	Name string

	// WebhookURL is the default POST target for webhook / slack / discord.
	WebhookURL string

	// Mailer names the mail.Manager transport for the mail channel.
	Mailer string

	// Extra carries driver-specific extension config.
	Extra map[string]string
}
