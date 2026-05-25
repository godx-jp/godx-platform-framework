package azure_test

import (
	"context"
	"errors"
	"testing"

	"github.com/godx-jp/godx-platform-framework/storage/driver"
	_ "github.com/godx-jp/godx-platform-framework/storage/drivers/azure"
)

func TestAzure_StubReturnsNotImplemented(t *testing.T) {
	_, err := driver.New(context.Background(), driver.Spec{Name: driver.DriverAzure, Bucket: "b"})
	if !errors.Is(err, driver.ErrNotImplemented) {
		t.Fatalf("want ErrNotImplemented, got %v", err)
	}
}
