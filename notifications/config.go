package notifications

import (
	"fmt"
	"os"
	"strings"

	ndriver "github.com/godx-jp/godx-platform-framework/notifications/driver"
)

const (
	EnvDefault     = "NOTIFICATIONS_DEFAULT"
	EnvChannels    = "NOTIFICATIONS_CHANNELS"
	EnvWebhookURL  = "NOTIFICATIONS_WEBHOOK_URL"
	EnvMailMailer  = "NOTIFICATIONS_MAIL_MAILER"
)

// ChannelConfig configures one named notification channel.
type ChannelConfig struct {
	Driver string
	Spec   ndriver.Spec
}

// Config configures the notifications module.
type Config struct {
	Default       string
	Channels      map[string]ChannelConfig
	DatabaseStore ndriver.DatabaseStore
}

// LoadConfigFromEnv builds a Config from the process environment.
func LoadConfigFromEnv() Config {
	def := strings.TrimSpace(os.Getenv(EnvDefault))
	if def == "" {
		def = ndriver.DriverLog
	}
	names := splitCSV(os.Getenv(EnvChannels))
	if len(names) == 0 {
		names = []string{def}
	}
	channels := make(map[string]ChannelConfig, len(names))
	for _, name := range names {
		channels[name] = ChannelConfig{
			Driver: name,
			Spec:   loadSpec(name),
		}
	}
	return Config{Default: def, Channels: channels}
}

func loadSpec(name string) ndriver.Spec {
	spec := ndriver.Spec{Name: name}
	switch name {
	case ndriver.DriverSlack, ndriver.DriverDiscord, ndriver.DriverWebhook:
		spec.WebhookURL = os.Getenv(EnvWebhookURL)
	case ndriver.DriverMail:
		spec.Mailer = os.Getenv(EnvMailMailer)
	}
	return spec
}

// Validate sanity-checks the Config.
func (c Config) Validate() error {
	if c.Default == "" {
		return fmt.Errorf("notifications: default channel name is required")
	}
	if len(c.Channels) == 0 {
		return fmt.Errorf("notifications: no channels configured")
	}
	if _, ok := c.Channels[c.Default]; !ok {
		return fmt.Errorf("notifications: default %q not present in Channels", c.Default)
	}
	for name, cc := range c.Channels {
		if strings.TrimSpace(cc.Driver) == "" {
			return fmt.Errorf("notifications: channel %q: driver is required", name)
		}
		if cc.Driver == ndriver.DriverDatabase && c.DatabaseStore == nil {
			return fmt.Errorf("notifications: database channel requires DatabaseStore in Config")
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
