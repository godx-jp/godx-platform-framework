package minio_test

import (
	"context"
	"strings"
	"testing"

	"github.com/godx-jp/godx-platform-framework/storage/driver"
	_ "github.com/godx-jp/godx-platform-framework/storage/drivers/minio"
)

// The full minio implementation is exercised by storage/drivers
// /internal/s3core via the fake API. These tests verify that the
// wrapper registers the driver under the expected name and enforces
// MinIO-specific required fields (bucket + endpoint).
func TestMinIO_RegistersUnderName(t *testing.T) {
	names := driver.Names()
	found := false
	for _, n := range names {
		if n == driver.DriverMinIO {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("minio driver must auto-register on blank import; have %v", names)
	}
}

func TestMinIO_BucketRequired(t *testing.T) {
	_, err := driver.New(context.Background(), driver.Spec{Name: driver.DriverMinIO, Endpoint: "http://localhost:9000"})
	if err == nil || !strings.Contains(err.Error(), "bucket is required") {
		t.Fatalf("want bucket-required error, got %v", err)
	}
}

func TestMinIO_EndpointRequired(t *testing.T) {
	_, err := driver.New(context.Background(), driver.Spec{Name: driver.DriverMinIO, Bucket: "b"})
	if err == nil || !strings.Contains(err.Error(), "endpoint is required") {
		t.Fatalf("want endpoint-required error, got %v", err)
	}
}
