package messaging

import (
	"fmt"
	"os"
	"strings"

	mdriver "github.com/godx-jp/godx-platform-framework/messaging/driver"
)

const (
	EnvDefault     = "MESSAGING_DEFAULT"
	EnvConnections = "MESSAGING_CONNECTIONS"
)

type ConnConfig struct {
	Driver mdriver.Spec
}

type Config struct {
	Default     string
	Connections map[string]ConnConfig
}

func LoadConfigFromEnv() Config {
	def := strings.TrimSpace(os.Getenv(EnvDefault))
	if def == "" {
		def = "platform"
	}
	names := splitCSV(os.Getenv(EnvConnections))
	if len(names) == 0 {
		names = []string{def}
	}
	conns := make(map[string]ConnConfig, len(names))
	for _, name := range names {
		driver := connEnv(name, "DRIVER", mdriver.DriverMemory)
		conns[name] = ConnConfig{Driver: mdriver.Spec{
			Name:          driver,
			SubjectPrefix: connEnv(name, "SUBJECT_PREFIX", ""),
			NATSURL:       connEnv(name, "NATS_URL", ""),
			StreamName:    connEnv(name, "JETSTREAM_STREAM", ""),
			KafkaTopic:    connEnv(name, "KAFKA_TOPIC", ""),
			KafkaBrokers:  splitCSV(connEnv(name, "KAFKA_BROKERS", "")),
		}}
	}
	return Config{Default: def, Connections: conns}
}

func connEnv(conn, field, def string) string {
	key := fmt.Sprintf("MESSAGING_CONN_%s_%s", strings.ToUpper(conn), field)
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
