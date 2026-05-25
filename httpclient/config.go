package httpclient

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	hdriver "github.com/godx-jp/godx-platform-framework/httpclient/driver"
)

const (
	EnvDefault         = "HTTPCLIENT_DEFAULT"
	EnvClients         = "HTTPCLIENT_CLIENTS"
	EnvBaseURL         = "HTTPCLIENT_BASE_URL"
	EnvTimeout         = "HTTPCLIENT_TIMEOUT"
	EnvOTelServiceName = "HTTPCLIENT_OTEL_SERVICE"
	EnvMaxRetries      = "HTTPCLIENT_MAX_RETRIES"
	EnvRetryBackoff    = "HTTPCLIENT_RETRY_BACKOFF"
)

type ClientConfig struct {
	Driver string
	Spec   hdriver.Spec
}

type Config struct {
	Default string
	Clients map[string]ClientConfig
}

func LoadConfigFromEnv() Config {
	def := strings.TrimSpace(os.Getenv(EnvDefault))
	if def == "" {
		def = hdriver.DriverStdlib
	}
	names := splitCSV(os.Getenv(EnvClients))
	if len(names) == 0 {
		names = []string{def}
	}
	clients := make(map[string]ClientConfig, len(names))
	for _, name := range names {
		clients[name] = ClientConfig{Driver: name, Spec: loadSpec(name)}
	}
	return Config{Default: def, Clients: clients}
}

func loadSpec(name string) hdriver.Spec {
	spec := hdriver.Spec{
		Name:            name,
		BaseURL:         os.Getenv(EnvBaseURL),
		OTelServiceName: os.Getenv(EnvOTelServiceName),
	}
	if v := os.Getenv(EnvTimeout); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			spec.Timeout = d
		}
	}
	if v := os.Getenv(EnvMaxRetries); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			spec.MaxRetries = n
		}
	}
	if v := os.Getenv(EnvRetryBackoff); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			spec.RetryBackoff = d
		}
	}
	return spec
}

func (c Config) Validate() error {
	if c.Default == "" {
		return fmt.Errorf("httpclient: default client name required")
	}
	if len(c.Clients) == 0 {
		return fmt.Errorf("httpclient: no clients configured")
	}
	if _, ok := c.Clients[c.Default]; !ok {
		return fmt.Errorf("httpclient: default %q not in Clients", c.Default)
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
