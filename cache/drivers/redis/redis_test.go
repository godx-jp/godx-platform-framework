package redis_test

import (
	"context"
	"strings"
	"testing"

	cdriver "github.com/godx-jp/godx-platform-framework/cache/driver"
	_ "github.com/godx-jp/godx-platform-framework/cache/drivers/redis"
)

// Full Redis round trip is exercised by the //go:build integration
// test against a live redis-server. These unit tests confirm
// registration and required-field validation only.
func TestRedis_RegistersUnderName(t *testing.T) {
	for _, n := range cdriver.Names() {
		if n == cdriver.DriverRedis {
			return
		}
	}
	t.Fatalf("redis driver must auto-register on blank import; have %v", cdriver.Names())
}

func TestRedis_AddressOrURLRequired(t *testing.T) {
	_, err := cdriver.New(context.Background(), cdriver.Spec{Name: cdriver.DriverRedis})
	if err == nil || !strings.Contains(err.Error(), "address (or URL) is required") {
		t.Fatalf("want address-required error, got %v", err)
	}
}

func TestRedis_BadURL(t *testing.T) {
	_, err := cdriver.New(context.Background(), cdriver.Spec{Name: cdriver.DriverRedis, URL: "not-a-redis-url"})
	if err == nil || !strings.Contains(err.Error(), "parse URL") {
		t.Fatalf("want parse-URL error, got %v", err)
	}
}
