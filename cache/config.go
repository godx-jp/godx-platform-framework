package cache

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	cdriver "github.com/godx-jp/godx-platform-framework/cache/driver"
)

// Environment variable names used by the cache module. Centralised here
// so callers can reference them programmatically (tests, documentation
// generators, infra tooling) instead of duplicating raw strings.
const (
	// EnvDefaultStore selects which configured store is returned by
	// Manager.Default(). Defaults to "memory".
	EnvDefaultStore = "CACHE_DEFAULT_STORE"

	// EnvStores is a comma-separated list of store names to register.
	// Each name resolves to per-store env vars prefixed with
	// CACHE_STORE_<NAME>_. When unset, the module registers the
	// default store only.
	EnvStores = "CACHE_STORES"

	// EnvGlobalPrefix is prepended to every key handed to every
	// store (per-store prefix is composed on top). Useful when one
	// Redis is shared between many services.
	EnvGlobalPrefix = "CACHE_PREFIX"

	// envStorePrefix is prefixed to per-store env var lookups, joined
	// with the uppercase store name and the field suffix.
	envStorePrefix = "CACHE_STORE_"
)

// StoreConfig is the configuration for a single named cache store.
// Maps roughly 1:1 to one entry in Laravel's config/cache.php "stores"
// array.
type StoreConfig struct {
	// Driver selects the implementation (memory, file, redis).
	// Required.
	Driver string

	// Prefix is prepended to every key written by this store. The
	// manager composes Config.GlobalPrefix + this prefix before
	// passing to the driver.
	Prefix string

	// DefaultTTL is the fallback TTL used by Store helpers that accept
	// a zero/negative ttl as "use the default". The driver itself
	// still treats ttl == 0 as "store forever".
	DefaultTTL time.Duration

	// ── file driver ──────────────────────────────────────────────
	Path string

	// ── redis driver ─────────────────────────────────────────────
	URL      string
	Address  string
	Username string
	Password string
	DB       int
	TLS      bool
}

// Config is the cache module configuration. The Module reads it from
// the environment via LoadConfigFromEnv.
type Config struct {
	// DefaultStore is the name returned by Manager.Default(). Must be
	// present in Stores.
	DefaultStore string

	// GlobalPrefix is prepended to every key across every store.
	GlobalPrefix string

	// Stores is the registered set of named stores.
	Stores map[string]StoreConfig
}

// LoadConfigFromEnv builds a Config from the process environment.
// Falls back to a single "memory" store when nothing is configured.
func LoadConfigFromEnv() Config {
	defaultStore := strings.TrimSpace(os.Getenv(EnvDefaultStore))
	if defaultStore == "" {
		defaultStore = "memory"
	}
	names := splitCSV(os.Getenv(EnvStores))
	if len(names) == 0 {
		names = []string{defaultStore}
	}
	stores := make(map[string]StoreConfig, len(names))
	for _, name := range names {
		stores[name] = LoadStoreConfigFromEnv(name)
	}
	return Config{
		DefaultStore: defaultStore,
		GlobalPrefix: strings.TrimSpace(os.Getenv(EnvGlobalPrefix)),
		Stores:       stores,
	}
}

// LoadStoreConfigFromEnv reads store-scoped configuration from env
// vars prefixed with CACHE_STORE_<NAME>_. Returns a StoreConfig with
// sensible defaults when nothing is set (Driver=memory).
func LoadStoreConfigFromEnv(name string) StoreConfig {
	seg := envSegment(name)
	get := func(suffix string) string {
		return strings.TrimSpace(os.Getenv(envStorePrefix + seg + "_" + suffix))
	}
	drv := get("DRIVER")
	if drv == "" {
		// Convenience: when the store name matches a known driver name
		// and DRIVER is unset, infer it. So `CACHE_STORES=redis` works
		// without `CACHE_STORE_REDIS_DRIVER=redis`.
		switch name {
		case cdriver.DriverMemory, cdriver.DriverFile, cdriver.DriverRedis:
			drv = name
		default:
			drv = cdriver.DriverMemory
		}
	}
	path := get("PATH")
	if path == "" && drv == cdriver.DriverFile {
		// Laravel-faithful default — storage/framework/cache.
		path = "./storage/framework/cache"
	}
	tls, _ := strconv.ParseBool(get("TLS"))
	db, _ := strconv.Atoi(get("DB"))
	ttl, _ := time.ParseDuration(get("DEFAULT_TTL"))

	return StoreConfig{
		Driver:     drv,
		Prefix:     get("PREFIX"),
		DefaultTTL: ttl,
		Path:       path,
		URL:        get("URL"),
		Address:    get("ADDRESS"),
		Username:   get("USERNAME"),
		Password:   get("PASSWORD"),
		DB:         db,
		TLS:        tls,
	}
}

// Validate sanity-checks the Config. Run at module init so
// misconfigurations crash on boot rather than at first cache call.
func (c Config) Validate() error {
	if c.DefaultStore == "" {
		return fmt.Errorf("cache: default store name is required")
	}
	if len(c.Stores) == 0 {
		return fmt.Errorf("cache: no stores configured")
	}
	if _, ok := c.Stores[c.DefaultStore]; !ok {
		return fmt.Errorf("cache: default store %q not present in Stores", c.DefaultStore)
	}
	for name, sc := range c.Stores {
		if err := sc.Validate(name); err != nil {
			return err
		}
	}
	return nil
}

// Validate checks one StoreConfig. The store name is included in
// error messages for clarity.
func (sc StoreConfig) Validate(storeName string) error {
	if strings.TrimSpace(sc.Driver) == "" {
		return fmt.Errorf("cache: store %q: driver is required", storeName)
	}
	switch sc.Driver {
	case cdriver.DriverFile:
		if strings.TrimSpace(sc.Path) == "" {
			return fmt.Errorf("cache: store %q (file): path is required", storeName)
		}
	case cdriver.DriverRedis:
		if strings.TrimSpace(sc.URL) == "" && strings.TrimSpace(sc.Address) == "" {
			return fmt.Errorf("cache: store %q (redis): URL or ADDRESS is required", storeName)
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

// envSegment normalises a store name to its env-var segment. Mirrors
// the storage module's disk-name normalisation: uppercase A–Z, 0–9,
// underscore preserved; "-" becomes "_"; everything else is dropped.
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
