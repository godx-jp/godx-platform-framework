package auth

import (
	"testing"

	adriver "github.com/godx-jp/godx-platform-framework/auth/driver"
)

func TestParseAPIKeysWithMetadataEnv(t *testing.T) {
	t.Setenv("AUTH_GUARD_API_KEY_SVC_ROLES", "admin,editor")
	t.Setenv("AUTH_GUARD_API_KEY_SVC_PERMISSIONS", "read,write")
	t.Setenv("AUTH_GUARD_API_KEY_SVC_ACTOR_KIND", "service")

	keys := parseAPIKeys("api", "svc:secret")
	entry, ok := keys["svc"]
	if !ok {
		t.Fatal("missing svc key")
	}
	if len(entry.Roles) != 2 || len(entry.Permissions) != 2 {
		t.Fatalf("roles=%v perms=%v", entry.Roles, entry.Permissions)
	}
	if entry.ActorKind != adriver.ActorService {
		t.Fatalf("kind=%q", entry.ActorKind)
	}
}

func TestInferDriverIntrospectByName(t *testing.T) {
	t.Setenv("AUTH_GUARD_INTROSPECT_DRIVER", "")
	if got := inferDriver("introspect"); got != adriver.DriverIntrospect {
		t.Fatalf("got %q", got)
	}
}

func TestSplitCSVEmptyAndSpaces(t *testing.T) {
	if splitCSV("") != nil {
		t.Fatal("empty csv should be nil")
	}
	if got := splitCSV(" a , , b "); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("got %v", got)
	}
}

func TestEnvKeyHyphenated(t *testing.T) {
	if envKey("my-guard") != "MY_GUARD" {
		t.Fatalf("got %q", envKey("my-guard"))
	}
}

func TestConfigValidateEmptyGuards(t *testing.T) {
	cfg := Config{Default: "jwt", Guards: map[string]GuardConfig{}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for empty guards")
	}
}
