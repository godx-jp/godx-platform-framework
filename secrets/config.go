package secrets

import (
	"fmt"
	"os"
	"strings"

	sdriver "github.com/godx-jp/godx-platform-framework/secrets/driver"
)

const (
	// EnvDefault selects the default secrets store.
	EnvDefault = "SECRETS_DEFAULT"

	// EnvStores is a comma-separated list of stores to register —
	// each name implies a driver of the same name unless overridden
	// by SECRETS_<NAME>_DRIVER. When unset, only the default store
	// is registered.
	EnvStores = "SECRETS_STORES"

	// EnvPrefix prefixes every key on the default store.
	EnvPrefix = "SECRETS_PREFIX"

	// Env-driver options.
	EnvEnvPrefix = "SECRETS_ENV_PREFIX"

	// File-driver options.
	EnvFilePath = "SECRETS_FILE_PATH"

	// Vault-driver options.
	EnvVaultAddr  = "SECRETS_VAULT_ADDR"
	EnvVaultToken = "SECRETS_VAULT_TOKEN"
	EnvVaultMount = "SECRETS_VAULT_KV_MOUNT"

	// GCPSM-driver options.
	EnvGCPSMProject = "SECRETS_GCPSM_PROJECT"

	// AWSSM-driver options.
	EnvAWSSMRegion = "SECRETS_AWSSM_REGION"
)

// StoreConfig configures one named secrets store.
type StoreConfig struct {
	Driver string
	Spec   sdriver.Spec
}

// Config configures the secrets module.
type Config struct {
	Default string
	Stores  map[string]StoreConfig
}

// LoadConfigFromEnv builds a Config from the process environment.
// Falls back to a single env-driver store when nothing is configured.
func LoadConfigFromEnv() Config {
	def := strings.TrimSpace(os.Getenv(EnvDefault))
	if def == "" {
		def = sdriver.DriverEnv
	}
	names := splitCSV(os.Getenv(EnvStores))
	if len(names) == 0 {
		names = []string{def}
	}
	stores := make(map[string]StoreConfig, len(names))
	for _, name := range names {
		stores[name] = StoreConfig{
			Driver: name,
			Spec:   loadSpec(name),
		}
	}
	return Config{Default: def, Stores: stores}
}

func loadSpec(name string) sdriver.Spec {
	spec := sdriver.Spec{Name: name, Prefix: os.Getenv(EnvPrefix)}
	switch name {
	case sdriver.DriverEnv:
		if v := os.Getenv(EnvEnvPrefix); v != "" {
			spec.Prefix = v
		}
	case sdriver.DriverFile:
		spec.Path = os.Getenv(EnvFilePath)
	case sdriver.DriverVault:
		spec.Address = os.Getenv(EnvVaultAddr)
		spec.Token = os.Getenv(EnvVaultToken)
		spec.KVMount = os.Getenv(EnvVaultMount)
	case sdriver.DriverGCPSM:
		spec.Project = os.Getenv(EnvGCPSMProject)
	case sdriver.DriverAWSSM:
		spec.Region = os.Getenv(EnvAWSSMRegion)
	}
	return spec
}

// Validate sanity-checks the Config.
func (c Config) Validate() error {
	if c.Default == "" {
		return fmt.Errorf("secrets: default store name is required")
	}
	if len(c.Stores) == 0 {
		return fmt.Errorf("secrets: no stores configured")
	}
	if _, ok := c.Stores[c.Default]; !ok {
		return fmt.Errorf("secrets: default store %q not present in Stores", c.Default)
	}
	for name, sc := range c.Stores {
		if strings.TrimSpace(sc.Driver) == "" {
			return fmt.Errorf("secrets: store %q: driver is required", name)
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
