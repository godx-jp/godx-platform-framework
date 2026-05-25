package storage

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/godx-jp/godx-platform-framework/storage/driver"
)

// Environment variable names used by the storage module. Centralised
// here so callers can reference them programmatically (tests,
// documentation generators, infra tooling) instead of duplicating raw
// strings.
const (
	// EnvDefaultDisk selects which configured disk is returned by
	// Manager.Default(). Defaults to "local".
	EnvDefaultDisk = "STORAGE_DEFAULT_DISK"

	// EnvDisks is a comma-separated list of disk names to register.
	// Each name resolves to per-disk env vars prefixed with
	// STORAGE_DISK_<NAME>_. When unset, the module registers the
	// default disk only.
	EnvDisks = "STORAGE_DISKS"

	// envDiskPrefix is prefixed to per-disk env var lookups, joined
	// with the uppercase disk name and the field suffix.
	envDiskPrefix = "STORAGE_DISK_"
)

// DiskConfig is the configuration for a single named disk. Maps 1:1 to
// Laravel's per-disk entry in config/filesystems.php.
type DiskConfig struct {
	// Driver selects the implementation (local, memory, s3, gcs,
	// azure, minio). Required.
	Driver string

	// Root is the filesystem root path for the local driver. Defaults
	// to "./storage".
	Root string

	// DefaultVisibility is applied to writes that do not specify a
	// visibility explicitly. Defaults to VisibilityPrivate.
	DefaultVisibility driver.Visibility

	// Object-store fields. Ignored by local/memory drivers.
	Bucket       string
	Region       string
	Endpoint     string
	UsePathStyle bool
	AccessKey    string
	SecretKey    string
	SessionToken string

	// PublicURL is the base URL prepended to keys when Disk.URL is
	// called.
	PublicURL string

	// Extra carries driver-specific key/value extension config.
	Extra map[string]string
}

// Config is the storage module configuration. The Module reads it from
// the environment via LoadConfigFromEnv.
type Config struct {
	// DefaultDisk is the name returned by Manager.Default(). Must be
	// present in Disks.
	DefaultDisk string

	// Disks is the registered set of named disks.
	Disks map[string]DiskConfig
}

// LoadConfigFromEnv builds a Config from the process environment.
// Falls back to a single "local" disk rooted at "./storage/app/private"
// when nothing is configured — matches Laravel's out-of-the-box default
// disk (`storage_path('app/private')`).
func LoadConfigFromEnv() Config {
	defaultDisk := strings.TrimSpace(os.Getenv(EnvDefaultDisk))
	if defaultDisk == "" {
		defaultDisk = "local"
	}

	names := splitCSV(os.Getenv(EnvDisks))
	if len(names) == 0 {
		names = []string{defaultDisk}
	}

	disks := make(map[string]DiskConfig, len(names))
	for _, name := range names {
		disks[name] = LoadDiskConfigFromEnv(name)
	}
	return Config{DefaultDisk: defaultDisk, Disks: disks}
}

// LoadDiskConfigFromEnv reads disk-scoped configuration from env vars
// prefixed with STORAGE_DISK_<NAME>_. Returns a DiskConfig with sensible
// defaults when nothing is set (Driver=local, Root=./storage).
func LoadDiskConfigFromEnv(name string) DiskConfig {
	seg := envSegment(name)
	get := func(suffix string) string {
		return strings.TrimSpace(os.Getenv(envDiskPrefix + seg + "_" + suffix))
	}
	driverName := get("DRIVER")
	if driverName == "" {
		driverName = "local"
	}
	root := get("ROOT")
	if root == "" && driverName == "local" {
		// Laravel-faithful default: the "local" disk is the private
		// per-app directory under ./storage/app/private. The matching
		// public disk (visibility=public, public URL) is conventionally
		// rooted at ./storage/app/public. See docs/modules/storage.md.
		root = "./storage/app/private"
	}
	vis := driver.Visibility(strings.ToLower(get("VISIBILITY")))
	usePathStyle, _ := strconv.ParseBool(get("USE_PATH_STYLE"))

	return DiskConfig{
		Driver:            driverName,
		Root:              root,
		DefaultVisibility: vis,
		Bucket:            get("BUCKET"),
		Region:            get("REGION"),
		Endpoint:          get("ENDPOINT"),
		UsePathStyle:      usePathStyle,
		AccessKey:         get("ACCESS_KEY"),
		SecretKey:         get("SECRET_KEY"),
		SessionToken:      get("SESSION_TOKEN"),
		PublicURL:         get("PUBLIC_URL"),
	}
}

// Validate sanity-checks the Config. Run at module init so
// misconfigurations crash on boot rather than at first write.
func (c Config) Validate() error {
	if c.DefaultDisk == "" {
		return fmt.Errorf("storage: default disk name is required")
	}
	if len(c.Disks) == 0 {
		return fmt.Errorf("storage: no disks configured")
	}
	if _, ok := c.Disks[c.DefaultDisk]; !ok {
		return fmt.Errorf("storage: default disk %q not present in Disks", c.DefaultDisk)
	}
	for name, dc := range c.Disks {
		if err := dc.Validate(name); err != nil {
			return err
		}
	}
	return nil
}

// Validate checks one DiskConfig. The disk name is included in error
// messages for clarity.
func (dc DiskConfig) Validate(diskName string) error {
	if strings.TrimSpace(dc.Driver) == "" {
		return fmt.Errorf("storage: disk %q: driver is required", diskName)
	}
	switch dc.Driver {
	case driver.DriverLocal:
		if strings.TrimSpace(dc.Root) == "" {
			return fmt.Errorf("storage: disk %q (local): root is required", diskName)
		}
	case driver.DriverS3, driver.DriverGCS, driver.DriverAzure, driver.DriverMinIO:
		if strings.TrimSpace(dc.Bucket) == "" {
			return fmt.Errorf("storage: disk %q (%s): bucket is required", diskName, dc.Driver)
		}
	}
	if dc.DefaultVisibility != "" && !dc.DefaultVisibility.IsValid() {
		return fmt.Errorf("storage: disk %q: invalid visibility %q (want public|private)", diskName, dc.DefaultVisibility)
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

// envSegment normalises a disk name to its env-var segment.
//
// Mirrors observability's channel name normalisation: uppercase letters,
// digits, and underscore are preserved; "-" becomes "_"; everything
// else is dropped. Empty input is rejected upstream by Validate.
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
