package redis_test

import (
	"context"
	"strings"
	"testing"

	qdriver "github.com/godx-jp/godx-platform-framework/queue/driver"
	reddrv "github.com/godx-jp/godx-platform-framework/queue/drivers/redis"
)

func TestRegisteredOnImport(t *testing.T) {
	if qdriver.Lookup(qdriver.DriverRedis) == nil {
		t.Fatal("redis driver not registered")
	}
}

func TestNewFromSpecRequiresAddress(t *testing.T) {
	_, err := reddrv.NewFromSpec(context.Background(), qdriver.Spec{Name: qdriver.DriverRedis})
	if err == nil {
		t.Fatal("expected error without URL/ADDRESS")
	}
	if !strings.Contains(err.Error(), "URL or ADDRESS") {
		t.Fatalf("error=%v", err)
	}
}

func TestRedisKey(t *testing.T) {
	if got := reddrv.RedisKey("pfx:", "list:jobs"); got != "pfx:list:jobs" {
		t.Fatalf("got %q", got)
	}
}
