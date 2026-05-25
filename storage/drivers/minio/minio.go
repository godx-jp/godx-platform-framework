// Package minio wires a MinIO-backed storage driver into the registry.
// MinIO speaks the S3 API, so the driver shares most of its
// implementation with drivers/s3 but uses different defaults
// (UsePathStyle=true, explicit Endpoint required).
//
// HEAVY driver — opt in with a blank import:
//
//	import _ "github.com/godx-jp/godx-platform-framework/storage/drivers/minio"
//
// Full implementation lands in a v0.6.x patch release.
package minio

import (
	"context"
	"fmt"

	"github.com/godx-jp/godx-platform-framework/storage/driver"
)

func init() {
	driver.Register(driver.DriverMinIO, construct)
}

const Name = driver.DriverMinIO

func construct(_ context.Context, _ driver.Spec) (driver.Driver, error) {
	return nil, fmt.Errorf(
		"%w: storage/drivers/minio — full implementation landing in v0.6.x",
		driver.ErrNotImplemented,
	)
}
