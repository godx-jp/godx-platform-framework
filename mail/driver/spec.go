package driver

// Spec is the uniform input to every mail driver constructor.
type Spec struct {
	// Name is the driver name (log, smtp, ses, sendgrid, mailgun, postmark).
	Name string

	// From is the default sender address applied by the Manager when
	// a Mailer does not override it.
	From string

	// ── smtp driver ──────────────────────────────────────────────
	Host     string
	Port     int
	Username string
	Password string
	TLS      bool

	// ── ses driver ───────────────────────────────────────────────
	Region string

	// ── sendgrid / mailgun / postmark ────────────────────────────
	APIKey string
	Domain string // mailgun only

	// Extra carries driver-specific extension config.
	Extra map[string]string
}
