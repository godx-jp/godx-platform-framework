package s3_test

import (
	"context"
	"strings"
	"testing"

	"github.com/godx-jp/godx-platform-framework/storage/driver"
	_ "github.com/godx-jp/godx-platform-framework/storage/drivers/s3"
)

// The full s3 implementation is exercised by storage/drivers/internal
// /s3core via the fake API. These tests verify that the wrapper
// registers the driver under the expected name and surfaces clear
// errors for required-field misconfiguration.
func TestS3_RegistersUnderName(t *testing.T) {
	names := driver.Names()
	found := false
	for _, n := range names {
		if n == driver.DriverS3 {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("s3 driver must auto-register on blank import; have %v", names)
	}
}

func TestS3_BucketRequired(t *testing.T) {
	_, err := driver.New(context.Background(), driver.Spec{Name: driver.DriverS3, Region: "ap-northeast-1"})
	if err == nil || !strings.Contains(err.Error(), "bucket is required") {
		t.Fatalf("want bucket-required error, got %v", err)
	}
}

func TestS3_RegionRequired(t *testing.T) {
	_, err := driver.New(context.Background(), driver.Spec{Name: driver.DriverS3, Bucket: "b"})
	if err == nil || !strings.Contains(err.Error(), "region is required") {
		t.Fatalf("want region-required error, got %v", err)
	}
}
