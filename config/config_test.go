package config

import (
	"strings"
	"testing"

	cdriver "github.com/godx-jp/godx-platform-framework/config/driver"
)

func TestLoadConfigFromEnvDefaults(t *testing.T) {
	for _, k := range []string{EnvSources, EnvAutoEnv, EnvEnvPrefix} {
		t.Setenv(k, "")
	}
	cfg := LoadConfigFromEnv()
	if !cfg.AutoEnv {
		t.Fatalf("AutoEnv should default to true")
	}
	if len(cfg.Sources) != 0 {
		t.Fatalf("no sources when CONFIG_SOURCES empty")
	}
}

func TestLoadConfigFromEnvWithSources(t *testing.T) {
	t.Setenv(EnvSources, "file,remote")
	t.Setenv("CONFIG_SOURCE_FILE_PATH", "/etc/app.yaml")
	t.Setenv("CONFIG_SOURCE_REMOTE_URL", "etcd://x:2379")
	cfg := LoadConfigFromEnv()
	if len(cfg.Sources) != 2 {
		t.Fatalf("want 2 sources, got %d", len(cfg.Sources))
	}
	if cfg.Sources[0].Config.Path != "/etc/app.yaml" {
		t.Fatalf("file path not parsed")
	}
	if cfg.Sources[1].Config.URL != "etcd://x:2379" {
		t.Fatalf("remote url not parsed")
	}
}

func TestValidateRejectsEmptyChain(t *testing.T) {
	cfg := Config{AutoEnv: false}
	if err := cfg.Validate(); err == nil {
		t.Fatalf("should reject empty chain with AutoEnv=false")
	}
}

func TestValidateSourceDriverRequired(t *testing.T) {
	cfg := Config{
		AutoEnv: true,
		Sources: []NamedSourceConfig{
			{Name: "x", Config: SourceConfig{}},
		},
	}
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "driver is required") {
		t.Fatalf("want driver-required error, got %v", err)
	}
}

func TestValidateFileRequiresPath(t *testing.T) {
	cfg := Config{
		AutoEnv: true,
		Sources: []NamedSourceConfig{
			{Name: "f", Config: SourceConfig{Driver: cdriver.DriverFile}},
		},
	}
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "PATH is required") {
		t.Fatalf("want PATH-required error, got %v", err)
	}
}

func TestValidateRemoteRequiresAddress(t *testing.T) {
	cfg := Config{
		AutoEnv: true,
		Sources: []NamedSourceConfig{
			{Name: "r", Config: SourceConfig{Driver: cdriver.DriverRemote}},
		},
	}
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "URL or ADDRESS is required") {
		t.Fatalf("want ADDRESS-required error, got %v", err)
	}
}

func TestEnvSegmentNormalisation(t *testing.T) {
	cases := map[string]string{
		"db":          "DB",
		"my-svc":      "MY_SVC",
		"dot.name":    "DOTNAME",
		"weird name!": "WEIRDNAME",
	}
	for in, want := range cases {
		if got := envSegment(in); got != want {
			t.Errorf("envSegment(%q) = %q, want %q", in, got, want)
		}
	}
}
