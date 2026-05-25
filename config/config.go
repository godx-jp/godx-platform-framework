package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	cdriver "github.com/godx-jp/godx-platform-framework/config/driver"
)

// Environment variable names used by the config module.
const (
	// EnvSources is a comma-separated list of source names to register
	// in merge order. Each name resolves to per-source env vars
	// prefixed with CONFIG_SOURCE_<NAME>_. When unset, the module
	// registers a single env source.
	EnvSources = "CONFIG_SOURCES"
	// EnvAutoEnv toggles auto-registration of the os-env source on
	// top of the configured chain so process env always wins.
	// Defaults to true.
	EnvAutoEnv = "CONFIG_AUTO_ENV"
	// EnvEnvPrefix scopes the implicit os-env source to vars sharing
	// this prefix. Strip-on-load is done by the env driver.
	EnvEnvPrefix = "CONFIG_ENV_PREFIX"

	envSourcePrefix = "CONFIG_SOURCE_"
)

// SourceConfig is the configuration for a single named source.
type SourceConfig struct {
	// Driver selects the implementation (env, file, remote, …).
	Driver string

	// Prefix scopes the source. See driver.Spec.Prefix.
	Prefix string

	// ── file driver ──────────────────────────────────────────────
	Path     string
	Format   string
	Optional bool

	// ── remote driver ────────────────────────────────────────────
	Address string
	URL     string
	Token   string
}

// Config is the config module's own configuration. Loaded from env
// by LoadConfigFromEnv.
type Config struct {
	// Sources is the ordered list of named sources. Earlier entries
	// are merged first, later entries override them.
	Sources []NamedSourceConfig

	// AutoEnv adds an implicit os-env source after every configured
	// source so process env always wins. Defaults to true.
	AutoEnv bool

	// AutoEnvPrefix scopes the implicit os-env source when AutoEnv
	// is true. Empty means "every env var".
	AutoEnvPrefix string
}

// NamedSourceConfig pairs a source name with its config.
type NamedSourceConfig struct {
	Name   string
	Config SourceConfig
}

// LoadConfigFromEnv builds a Config from the process environment.
func LoadConfigFromEnv() Config {
	names := splitCSV(os.Getenv(EnvSources))
	sources := make([]NamedSourceConfig, 0, len(names))
	for _, name := range names {
		sources = append(sources, NamedSourceConfig{
			Name:   name,
			Config: LoadSourceConfigFromEnv(name),
		})
	}
	autoEnv := true
	if v := strings.TrimSpace(os.Getenv(EnvAutoEnv)); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			autoEnv = b
		}
	}
	return Config{
		Sources:       sources,
		AutoEnv:       autoEnv,
		AutoEnvPrefix: strings.TrimSpace(os.Getenv(EnvEnvPrefix)),
	}
}

// LoadSourceConfigFromEnv reads source-scoped configuration from
// env vars prefixed with CONFIG_SOURCE_<NAME>_.
func LoadSourceConfigFromEnv(name string) SourceConfig {
	seg := envSegment(name)
	get := func(suffix string) string {
		return strings.TrimSpace(os.Getenv(envSourcePrefix + seg + "_" + suffix))
	}
	drv := get("DRIVER")
	if drv == "" {
		switch name {
		case cdriver.DriverEnv, cdriver.DriverFile, cdriver.DriverRemote, cdriver.DriverStatic:
			drv = name
		default:
			drv = cdriver.DriverFile
		}
	}
	optional, _ := strconv.ParseBool(get("OPTIONAL"))
	return SourceConfig{
		Driver:   drv,
		Prefix:   get("PREFIX"),
		Path:     get("PATH"),
		Format:   get("FORMAT"),
		Optional: optional,
		Address:  get("ADDRESS"),
		URL:      get("URL"),
		Token:    get("TOKEN"),
	}
}

// Validate sanity-checks the Config.
func (c Config) Validate() error {
	if len(c.Sources) == 0 && !c.AutoEnv {
		return fmt.Errorf("config: no sources configured and AutoEnv is disabled")
	}
	for _, ns := range c.Sources {
		if strings.TrimSpace(ns.Name) == "" {
			return fmt.Errorf("config: source name is required")
		}
		if err := ns.Config.Validate(ns.Name); err != nil {
			return err
		}
	}
	return nil
}

// Validate checks one SourceConfig.
func (sc SourceConfig) Validate(name string) error {
	if strings.TrimSpace(sc.Driver) == "" {
		return fmt.Errorf("config: source %q: driver is required", name)
	}
	switch sc.Driver {
	case cdriver.DriverFile:
		if strings.TrimSpace(sc.Path) == "" {
			return fmt.Errorf("config: source %q (file): PATH is required", name)
		}
	case cdriver.DriverRemote:
		if strings.TrimSpace(sc.URL) == "" && strings.TrimSpace(sc.Address) == "" {
			return fmt.Errorf("config: source %q (remote): URL or ADDRESS is required", name)
		}
	}
	return nil
}

// ToSpec converts a SourceConfig to a driver.Spec.
func (sc SourceConfig) ToSpec(name string) cdriver.Spec {
	return cdriver.Spec{
		Name:     sc.Driver,
		Prefix:   sc.Prefix,
		Path:     sc.Path,
		Format:   sc.Format,
		Optional: sc.Optional,
		Address:  sc.Address,
		URL:      sc.URL,
		Token:    sc.Token,
	}
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

// envSegment normalises a source name to its env-var segment.
func envSegment(name string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(name) {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		case r == '-':
			b.WriteByte('_')
		}
	}
	return b.String()
}
