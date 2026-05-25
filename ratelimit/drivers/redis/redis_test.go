package redis_test

import (
	"context"
	"strings"
	"testing"

	rdriver "github.com/godx-jp/godx-platform-framework/ratelimit/driver"
	reddrv "github.com/godx-jp/godx-platform-framework/ratelimit/drivers/redis"
)

func TestRegisteredOnImport(t *testing.T) {
	if rdriver.Lookup(rdriver.DriverRedis) == nil {
		t.Fatalf("redis driver not registered")
	}
}

func TestNewFromSpecRequiresAddress(t *testing.T) {
	_, err := reddrv.NewFromSpec(context.Background(), rdriver.Spec{Name: rdriver.DriverRedis})
	if err == nil {
		t.Fatalf("expected error without URL/ADDRESS")
	}
	if !strings.Contains(err.Error(), "URL or ADDRESS") {
		t.Fatalf("error=%v", err)
	}
}

func TestRedisKey(t *testing.T) {
	if got := reddrv.RedisKey("pfx:", "user-1"); got != "pfx:user-1" {
		t.Fatalf("got %q", got)
	}
}
