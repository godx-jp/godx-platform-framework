package cache_test

import (
	"strings"
	"testing"
	"time"

	"github.com/godx-jp/godx-platform-framework/cache"
	cdriver "github.com/godx-jp/godx-platform-framework/cache/driver"
)

func TestLoadConfigFromEnv_DefaultsToInMemoryStore(t *testing.T) {
	t.Setenv(cache.EnvDefaultStore, "")
	t.Setenv(cache.EnvStores, "")
	cfg := cache.LoadConfigFromEnv()
	if cfg.DefaultStore != "memory" {
		t.Fatalf("default store: %q", cfg.DefaultStore)
	}
	if _, ok := cfg.Stores["memory"]; !ok {
		t.Fatalf("memory store missing: %+v", cfg.Stores)
	}
	if cfg.Stores["memory"].Driver != cdriver.DriverMemory {
		t.Fatalf("memory store driver: %q", cfg.Stores["memory"].Driver)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
}

func TestLoadConfigFromEnv_ParsesMultiStore(t *testing.T) {
	t.Setenv(cache.EnvDefaultStore, "primary")
	t.Setenv(cache.EnvStores, "primary, sessions")
	t.Setenv(cache.EnvGlobalPrefix, "svc:")
	t.Setenv("CACHE_STORE_PRIMARY_DRIVER", "memory")
	t.Setenv("CACHE_STORE_PRIMARY_PREFIX", "primary:")
	t.Setenv("CACHE_STORE_PRIMARY_DEFAULT_TTL", "30s")
	t.Setenv("CACHE_STORE_SESSIONS_DRIVER", "file")
	t.Setenv("CACHE_STORE_SESSIONS_PATH", "/tmp/sess-cache")

	cfg := cache.LoadConfigFromEnv()
	if cfg.GlobalPrefix != "svc:" {
		t.Fatalf("global prefix: %q", cfg.GlobalPrefix)
	}
	if cfg.Stores["primary"].DefaultTTL != 30*time.Second {
		t.Fatalf("primary ttl: %v", cfg.Stores["primary"].DefaultTTL)
	}
	if cfg.Stores["sessions"].Path != "/tmp/sess-cache" {
		t.Fatalf("sessions path: %q", cfg.Stores["sessions"].Path)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
}

func TestLoadConfigFromEnv_InfersDriverFromStoreName(t *testing.T) {
	t.Setenv(cache.EnvDefaultStore, "redis")
	t.Setenv(cache.EnvStores, "redis")
	t.Setenv("CACHE_STORE_REDIS_ADDRESS", "127.0.0.1:6379")
	cfg := cache.LoadConfigFromEnv()
	if cfg.Stores["redis"].Driver != cdriver.DriverRedis {
		t.Fatalf("expected store named 'redis' to default to driver=redis, got %q", cfg.Stores["redis"].Driver)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
}

func TestLoadDiskConfig_NormalizesHyphenatedName(t *testing.T) {
	t.Setenv("CACHE_STORE_USER_SESSIONS_DRIVER", "memory")
	sc := cache.LoadStoreConfigFromEnv("user-sessions")
	if sc.Driver != cdriver.DriverMemory {
		t.Fatalf("driver: %q", sc.Driver)
	}
}

func TestConfig_FileDriverRequiresPath(t *testing.T) {
	cfg := cache.Config{
		DefaultStore: "f",
		Stores: map[string]cache.StoreConfig{
			"f": {Driver: cdriver.DriverFile},
		},
	}
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "path is required") {
		t.Fatalf("want path-required error, got %v", err)
	}
}

func TestConfig_RedisDriverRequiresAddressOrURL(t *testing.T) {
	cfg := cache.Config{
		DefaultStore: "r",
		Stores: map[string]cache.StoreConfig{
			"r": {Driver: cdriver.DriverRedis},
		},
	}
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "URL or ADDRESS") {
		t.Fatalf("want url-or-address error, got %v", err)
	}
}
