package gcs_test

import (
	"context"
	"errors"
	"testing"

	"github.com/godx-jp/godx-platform-framework/storage/driver"
	_ "github.com/godx-jp/godx-platform-framework/storage/drivers/gcs"
)

func TestGCS_StubReturnsNotImplemented(t *testing.T) {
	_, err := driver.New(context.Background(), driver.Spec{Name: driver.DriverGCS, Bucket: "b"})
	if !errors.Is(err, driver.ErrNotImplemented) {
		t.Fatalf("want ErrNotImplemented, got %v", err)
	}
}
