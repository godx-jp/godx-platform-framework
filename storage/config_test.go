package storage_test

import (
	"strings"
	"testing"

	"github.com/godx-jp/godx-platform-framework/storage"
	"github.com/godx-jp/godx-platform-framework/storage/driver"
)

func TestLoadConfigFromEnv_DefaultsToSingleLocalDisk(t *testing.T) {
	t.Setenv(storage.EnvDefaultDisk, "")
	t.Setenv(storage.EnvDisks, "")

	cfg := storage.LoadConfigFromEnv()
	if cfg.DefaultDisk != "local" {
		t.Fatalf("default disk: want local, got %q", cfg.DefaultDisk)
	}
	if _, ok := cfg.Disks["local"]; !ok {
		t.Fatalf("expected local disk in defaults, got %v", cfg.Disks)
	}
	if cfg.Disks["local"].Driver != "local" {
		t.Fatalf("default disk driver: want local, got %q", cfg.Disks["local"].Driver)
	}
	if cfg.Disks["local"].Root == "" {
		t.Fatalf("default disk root must not be empty")
	}
}

func TestLoadConfigFromEnv_HonoursMultipleDisks(t *testing.T) {
	t.Setenv(storage.EnvDefaultDisk, "uploads")
	t.Setenv(storage.EnvDisks, "uploads,cache")
	t.Setenv("STORAGE_DISK_UPLOADS_DRIVER", "local")
	t.Setenv("STORAGE_DISK_UPLOADS_ROOT", "/tmp/uploads")
	t.Setenv("STORAGE_DISK_UPLOADS_VISIBILITY", "public")
	t.Setenv("STORAGE_DISK_CACHE_DRIVER", "memory")

	cfg := storage.LoadConfigFromEnv()
	if cfg.DefaultDisk != "uploads" {
		t.Fatalf("default disk: want uploads, got %q", cfg.DefaultDisk)
	}
	if len(cfg.Disks) != 2 {
		t.Fatalf("want 2 disks, got %d", len(cfg.Disks))
	}
	up := cfg.Disks["uploads"]
	if up.Driver != "local" || up.Root != "/tmp/uploads" {
		t.Fatalf("uploads disk unexpected: %+v", up)
	}
	if up.DefaultVisibility != driver.VisibilityPublic {
		t.Fatalf("uploads visibility: %s", up.DefaultVisibility)
	}
	if cfg.Disks["cache"].Driver != "memory" {
		t.Fatalf("cache driver: %q", cfg.Disks["cache"].Driver)
	}
}

func TestConfigValidate(t *testing.T) {
	cases := []struct {
		name    string
		cfg     storage.Config
		wantErr string
	}{
		{
			name:    "missing default",
			cfg:     storage.Config{Disks: map[string]storage.DiskConfig{"local": {Driver: "local", Root: "/tmp"}}},
			wantErr: "default disk name is required",
		},
		{
			name:    "default not in disks",
			cfg:     storage.Config{DefaultDisk: "x", Disks: map[string]storage.DiskConfig{"local": {Driver: "local", Root: "/tmp"}}},
			wantErr: `default disk "x" not present`,
		},
		{
			name:    "local without root",
			cfg:     storage.Config{DefaultDisk: "local", Disks: map[string]storage.DiskConfig{"local": {Driver: "local"}}},
			wantErr: "root is required",
		},
		{
			name:    "s3 without bucket",
			cfg:     storage.Config{DefaultDisk: "x", Disks: map[string]storage.DiskConfig{"x": {Driver: "s3"}}},
			wantErr: "bucket is required",
		},
		{
			name:    "bad visibility",
			cfg:     storage.Config{DefaultDisk: "x", Disks: map[string]storage.DiskConfig{"x": {Driver: "local", Root: "/tmp", DefaultVisibility: "weird"}}},
			wantErr: `invalid visibility "weird"`,
		},
		{
			name: "ok",
			cfg:  storage.Config{DefaultDisk: "local", Disks: map[string]storage.DiskConfig{"local": {Driver: "local", Root: "/tmp"}}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("want ok, got %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want substring %q", err, tc.wantErr)
			}
		})
	}
}
