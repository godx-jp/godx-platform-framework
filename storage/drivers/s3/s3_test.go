package s3_test

import (
	"context"
	"errors"
	"testing"

	"github.com/godx-jp/godx-platform-framework/storage/driver"
	_ "github.com/godx-jp/godx-platform-framework/storage/drivers/s3"
)

// Heavy-driver stubs must register themselves so misconfigurations
// surface as a clear ErrNotImplemented rather than the unknown-driver
// hint. When the full implementation lands, this test flips to a
// real-construction smoke test.
func TestS3_StubReturnsNotImplemented(t *testing.T) {
	_, err := driver.New(context.Background(), driver.Spec{Name: driver.DriverS3, Bucket: "b"})
	if err == nil {
		t.Fatal("expected error from s3 stub")
	}
	if !errors.Is(err, driver.ErrNotImplemented) {
		t.Fatalf("want ErrNotImplemented, got %v", err)
	}
}
