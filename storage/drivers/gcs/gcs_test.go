package gcs_test

import (
	"context"
	"strings"
	"testing"

	"github.com/godx-jp/godx-platform-framework/storage/driver"
	_ "github.com/godx-jp/godx-platform-framework/storage/drivers/gcs"
)

// Full GCS round trip is exercised by the //go:build integration test
// against a fake-gcs-server emulator. These unit tests confirm
// registration and required-field validation only.
func TestGCS_RegistersUnderName(t *testing.T) {
	names := driver.Names()
	for _, n := range names {
		if n == driver.DriverGCS {
			return
		}
	}
	t.Fatalf("gcs driver must auto-register on blank import; have %v", names)
}

func TestGCS_BucketRequired(t *testing.T) {
	_, err := driver.New(context.Background(), driver.Spec{Name: driver.DriverGCS})
	if err == nil || !strings.Contains(err.Error(), "bucket is required") {
		t.Fatalf("want bucket-required error, got %v", err)
	}
}
