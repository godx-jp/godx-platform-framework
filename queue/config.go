package queue

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds env-driven queue wiring.
type Config struct {
	Default string
	Queues  map[string]QueueConfig
}

// QueueConfig describes one named queue connection.
type QueueConfig struct {
	Driver       string
	DefaultQueue string
	Workers      int
	QueueSize    int
	AWSRegion    string
	QueueURL     string
	Brokers      []string
	Topic        string
	GroupID      string
	NATSURL      string
	Subject      string
	StreamName   string
}

func (c Config) Validate() error {
	if len(c.Queues) == 0 {
		return fmt.Errorf("queue: at least one queue must be configured")
	}
	if c.Default == "" {
		return fmt.Errorf("queue: QUEUE_DEFAULT is required")
	}
	if _, ok := c.Queues[c.Default]; !ok {
		return fmt.Errorf("queue: QUEUE_DEFAULT %q is not in QUEUE_QUEUES", c.Default)
	}
	for name, qc := range c.Queues {
		if err := qc.Validate(name); err != nil {
			return err
		}
	}
	return nil
}

func (qc QueueConfig) Validate(name string) error {
	if qc.Driver == "" {
		return fmt.Errorf("queue: QUEUE_QUEUE_%s_DRIVER is required", strings.ToUpper(name))
	}
	return nil
}

func LoadConfigFromEnv() Config {
	cfg := Config{
		Default: env("QUEUE_DEFAULT", "default"),
		Queues:  make(map[string]QueueConfig),
	}
	names := splitCSV(env("QUEUE_QUEUES", "default"))
	for _, name := range names {
		prefix := "QUEUE_QUEUE_" + strings.ToUpper(name) + "_"
		qc := QueueConfig{
			Driver:       env(prefix+"DRIVER", "memory"),
			DefaultQueue: env(prefix+"DEFAULT", "default"),
			Workers:      envInt(prefix+"WORKERS", 1),
			QueueSize:    envInt(prefix+"SIZE", 256),
			AWSRegion:    env(prefix+"AWS_REGION", env("AWS_REGION", "")),
			QueueURL:     env(prefix+"URL", ""),
			Brokers:      splitCSV(env(prefix+"BROKERS", "")),
			Topic:        env(prefix+"TOPIC", ""),
			GroupID:      env(prefix+"GROUP", ""),
			NATSURL:      env(prefix+"NATS_URL", env("NATS_URL", "")),
			Subject:      env(prefix+"SUBJECT", ""),
			StreamName:   env(prefix+"STREAM", ""),
		}
		cfg.Queues[name] = qc
	}
	return cfg
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func splitCSV(s string) []string {
	if s == "" {
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
