package mail

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	mdriver "github.com/godx-jp/godx-platform-framework/mail/driver"
)

const (
	EnvDefault   = "MAIL_DEFAULT"
	EnvMailers   = "MAIL_MAILERS"
	EnvFrom      = "MAIL_FROM"
	EnvSMTPHost  = "MAIL_SMTP_HOST"
	EnvSMTPPort  = "MAIL_SMTP_PORT"
	EnvSMTPUser  = "MAIL_SMTP_USERNAME"
	EnvSMTPPass  = "MAIL_SMTP_PASSWORD"
	EnvSESRegion = "MAIL_SES_REGION"
	EnvAPIKey    = "MAIL_API_KEY"
	EnvDomain    = "MAIL_DOMAIN"
)

// MailerConfig configures one named mail transport.
type MailerConfig struct {
	Driver string
	Spec   mdriver.Spec
}

// Config configures the mail module.
type Config struct {
	Default string
	From    string
	Mailers map[string]MailerConfig
}

// LoadConfigFromEnv builds a Config from the process environment.
func LoadConfigFromEnv() Config {
	def := strings.TrimSpace(os.Getenv(EnvDefault))
	if def == "" {
		def = mdriver.DriverLog
	}
	names := splitCSV(os.Getenv(EnvMailers))
	if len(names) == 0 {
		names = []string{def}
	}
	mailers := make(map[string]MailerConfig, len(names))
	for _, name := range names {
		mailers[name] = MailerConfig{
			Driver: name,
			Spec:   loadSpec(name),
		}
	}
	return Config{
		Default: def,
		From:    os.Getenv(EnvFrom),
		Mailers: mailers,
	}
}

func loadSpec(name string) mdriver.Spec {
	spec := mdriver.Spec{
		Name: name,
		From: os.Getenv(EnvFrom),
	}
	switch name {
	case mdriver.DriverSMTP:
		spec.Host = os.Getenv(EnvSMTPHost)
		spec.Username = os.Getenv(EnvSMTPUser)
		spec.Password = os.Getenv(EnvSMTPPass)
		if v := os.Getenv(EnvSMTPPort); v != "" {
			if p, err := strconv.Atoi(v); err == nil {
				spec.Port = p
			}
		}
	case mdriver.DriverSES:
		spec.Region = os.Getenv(EnvSESRegion)
	case mdriver.DriverSendGrid, mdriver.DriverPostmark:
		spec.APIKey = os.Getenv(EnvAPIKey)
	case mdriver.DriverMailgun:
		spec.APIKey = os.Getenv(EnvAPIKey)
		spec.Domain = os.Getenv(EnvDomain)
	}
	return spec
}

// Validate sanity-checks the Config.
func (c Config) Validate() error {
	if c.Default == "" {
		return fmt.Errorf("mail: default mailer name is required")
	}
	if len(c.Mailers) == 0 {
		return fmt.Errorf("mail: no mailers configured")
	}
	if _, ok := c.Mailers[c.Default]; !ok {
		return fmt.Errorf("mail: default %q not present in Mailers", c.Default)
	}
	for name, mc := range c.Mailers {
		if strings.TrimSpace(mc.Driver) == "" {
			return fmt.Errorf("mail: mailer %q: driver is required", name)
		}
	}
	return nil
}

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
