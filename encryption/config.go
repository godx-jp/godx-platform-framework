package encryption

import (
	"fmt"
	"os"
	"strings"

	edriver "github.com/godx-jp/godx-platform-framework/encryption/driver"
)

const (
	// EnvDriver selects the cipher driver (aesgcm, chacha20poly1305).
	EnvDriver = "ENCRYPTION_DRIVER"
	// EnvKey is the primary key. Same format as Laravel APP_KEY:
	// "base64:<32-byte key>". Required.
	EnvKey = "ENCRYPTION_KEY"
	// EnvPrimaryKeyID overrides the primary key id. Defaults to "k1".
	EnvPrimaryKeyID = "ENCRYPTION_PRIMARY_KEY_ID"
	// EnvPreviousKeys is a comma-separated list of "id=base64key"
	// pairs registered under the cipher so old ciphertext stays
	// decryptable. Optional.
	EnvPreviousKeys = "ENCRYPTION_PREVIOUS_KEYS"
)

// KeyEntry is one registered key with its id.
type KeyEntry struct {
	ID  string
	Key []byte
}

// Config configures the encryption module.
type Config struct {
	Driver       string
	PrimaryKeyID string
	PrimaryKey   []byte
	Previous     []KeyEntry
}

// LoadConfigFromEnv builds a Config from the process environment.
func LoadConfigFromEnv() (Config, error) {
	drv := strings.TrimSpace(os.Getenv(EnvDriver))
	if drv == "" {
		drv = edriver.DriverAESGCM
	}
	raw := strings.TrimSpace(os.Getenv(EnvKey))
	if raw == "" {
		return Config{}, fmt.Errorf("encryption: %s is required (e.g. base64:<32-byte key>)", EnvKey)
	}
	pk, err := ParseKey(raw)
	if err != nil {
		return Config{}, err
	}
	pid := strings.TrimSpace(os.Getenv(EnvPrimaryKeyID))
	if pid == "" {
		pid = "k1"
	}
	cfg := Config{
		Driver:       drv,
		PrimaryKeyID: pid,
		PrimaryKey:   pk,
	}
	for _, raw := range splitCSV(os.Getenv(EnvPreviousKeys)) {
		i := strings.IndexByte(raw, '=')
		if i < 1 {
			return Config{}, fmt.Errorf("encryption: %s entry %q malformed (expected id=base64key)", EnvPreviousKeys, raw)
		}
		id := strings.TrimSpace(raw[:i])
		key, err := ParseKey(raw[i+1:])
		if err != nil {
			return Config{}, fmt.Errorf("encryption: %s entry %q: %w", EnvPreviousKeys, raw, err)
		}
		cfg.Previous = append(cfg.Previous, KeyEntry{ID: id, Key: key})
	}
	return cfg, nil
}

// Validate sanity-checks the Config.
func (c Config) Validate() error {
	if strings.TrimSpace(c.Driver) == "" {
		return fmt.Errorf("encryption: driver is required")
	}
	if strings.TrimSpace(c.PrimaryKeyID) == "" {
		return fmt.Errorf("encryption: primary key id is required")
	}
	if len(c.PrimaryKey) == 0 {
		return fmt.Errorf("encryption: primary key bytes are required")
	}
	for _, p := range c.Previous {
		if strings.TrimSpace(p.ID) == "" {
			return fmt.Errorf("encryption: previous key entry has empty id")
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
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}
