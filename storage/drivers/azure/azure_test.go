package azure_test

import (
	"context"
	"strings"
	"testing"

	"github.com/godx-jp/godx-platform-framework/storage/driver"
	_ "github.com/godx-jp/godx-platform-framework/storage/drivers/azure"
)

// Full Azure round trip is exercised by the //go:build integration
// test against Azurite. These unit tests confirm registration and
// required-field validation only.
func TestAzure_RegistersUnderName(t *testing.T) {
	names := driver.Names()
	for _, n := range names {
		if n == driver.DriverAzure {
			return
		}
	}
	t.Fatalf("azure driver must auto-register on blank import; have %v", names)
}

func TestAzure_BucketRequired(t *testing.T) {
	_, err := driver.New(context.Background(), driver.Spec{
		Name:     driver.DriverAzure,
		Endpoint: "https://example.blob.core.windows.net",
	})
	if err == nil || !strings.Contains(err.Error(), "bucket (container) is required") {
		t.Fatalf("want bucket-required error, got %v", err)
	}
}

func TestAzure_EndpointRequired(t *testing.T) {
	_, err := driver.New(context.Background(), driver.Spec{Name: driver.DriverAzure, Bucket: "c"})
	if err == nil || !strings.Contains(err.Error(), "endpoint is required") {
		t.Fatalf("want endpoint-required error, got %v", err)
	}
}
