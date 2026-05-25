package hashing

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	hdriver "github.com/godx-jp/godx-platform-framework/hashing/driver"
)

const (
	// EnvDefault selects the default hasher.
	EnvDefault = "HASHING_DEFAULT"
	// EnvHashers is a comma-separated list of hashers to register.
	// When empty, the module registers the default hasher only.
	EnvHashers = "HASHING_HASHERS"
	// EnvBcryptCost overrides the bcrypt cost factor (4..31).
	EnvBcryptCost = "HASHING_BCRYPT_COST"
	// EnvArgon2Time overrides the argon2id time cost (iterations).
	EnvArgon2Time = "HASHING_ARGON2ID_TIME"
	// EnvArgon2Memory overrides the argon2id memory cost in KiB.
	EnvArgon2Memory = "HASHING_ARGON2ID_MEMORY"
	// EnvArgon2Threads overrides the argon2id parallelism.
	EnvArgon2Threads = "HASHING_ARGON2ID_THREADS"
	// EnvScryptN overrides the scrypt N cost (power of 2).
	EnvScryptN = "HASHING_SCRYPT_N"
	// EnvScryptR overrides the scrypt r parameter.
	EnvScryptR = "HASHING_SCRYPT_R"
	// EnvScryptP overrides the scrypt p parameter.
	EnvScryptP = "HASHING_SCRYPT_P"
)

// HasherConfig configures one named hasher.
type HasherConfig struct {
	Driver string
	Spec   hdriver.Spec
}

// Config configures the hashing module.
type Config struct {
	Default string
	Hashers map[string]HasherConfig
}

// LoadConfigFromEnv builds a Config from the process environment.
// Falls back to a single bcrypt hasher when nothing is configured.
func LoadConfigFromEnv() Config {
	def := strings.TrimSpace(os.Getenv(EnvDefault))
	if def == "" {
		def = hdriver.DriverBcrypt
	}
	names := splitCSV(os.Getenv(EnvHashers))
	if len(names) == 0 {
		names = []string{def}
	}
	hashers := make(map[string]HasherConfig, len(names))
	for _, name := range names {
		hashers[name] = HasherConfig{
			Driver: inferDriver(name),
			Spec:   loadSpec(name),
		}
	}
	return Config{Default: def, Hashers: hashers}
}

func inferDriver(name string) string {
	switch name {
	case hdriver.DriverBcrypt, hdriver.DriverArgon2id, hdriver.DriverScrypt:
		return name
	}
	return hdriver.DriverBcrypt
}

func loadSpec(name string) hdriver.Spec {
	spec := hdriver.Spec{Name: inferDriver(name)}
	switch spec.Name {
	case hdriver.DriverBcrypt:
		if v := os.Getenv(EnvBcryptCost); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				spec.BcryptCost = n
			}
		}
	case hdriver.DriverArgon2id:
		if v := os.Getenv(EnvArgon2Time); v != "" {
			if n, err := strconv.ParseUint(v, 10, 32); err == nil {
				spec.Argon2Time = uint32(n)
			}
		}
		if v := os.Getenv(EnvArgon2Memory); v != "" {
			if n, err := strconv.ParseUint(v, 10, 32); err == nil {
				spec.Argon2Memory = uint32(n)
			}
		}
		if v := os.Getenv(EnvArgon2Threads); v != "" {
			if n, err := strconv.ParseUint(v, 10, 8); err == nil {
				spec.Argon2Threads = uint8(n)
			}
		}
	case hdriver.DriverScrypt:
		if v := os.Getenv(EnvScryptN); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				spec.ScryptN = n
			}
		}
		if v := os.Getenv(EnvScryptR); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				spec.ScryptR = n
			}
		}
		if v := os.Getenv(EnvScryptP); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				spec.ScryptP = n
			}
		}
	}
	return spec
}

// Validate sanity-checks the Config.
func (c Config) Validate() error {
	if c.Default == "" {
		return fmt.Errorf("hashing: default hasher name is required")
	}
	if len(c.Hashers) == 0 {
		return fmt.Errorf("hashing: no hashers configured")
	}
	if _, ok := c.Hashers[c.Default]; !ok {
		return fmt.Errorf("hashing: default hasher %q not present in Hashers", c.Default)
	}
	for name, hc := range c.Hashers {
		if strings.TrimSpace(hc.Driver) == "" {
			return fmt.Errorf("hashing: hasher %q: driver is required", name)
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
