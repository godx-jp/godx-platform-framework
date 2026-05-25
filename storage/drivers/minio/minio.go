// Package minio wires a MinIO-backed storage driver into the registry.
// MinIO speaks the S3 API verbatim, so this driver shares its
// implementation with drivers/s3 via internal/s3core. The only
// differences are defaults:
//
//   - UsePathStyle is forced to true (MinIO requires path-style
//     addressing under the standard `:9000` endpoint).
//   - The Endpoint env var is REQUIRED — there is no public default
//     for MinIO the way AWS provides for S3.
//   - Region defaults to "us-east-1" when unset (MinIO uses it for
//     SigV4 only and accepts any value).
//
// HEAVY driver — opt in with a blank import:
//
//	import _ "github.com/godx-jp/godx-platform-framework/storage/drivers/minio"
//
// Typical local config:
//
//	STORAGE_DISK_LOCAL_S3_DRIVER=minio
//	STORAGE_DISK_LOCAL_S3_BUCKET=uploads
//	STORAGE_DISK_LOCAL_S3_ENDPOINT=http://localhost:9000
//	STORAGE_DISK_LOCAL_S3_ACCESS_KEY=minioadmin
//	STORAGE_DISK_LOCAL_S3_SECRET_KEY=minioadmin
//	# STORAGE_DISK_LOCAL_S3_PUBLIC_URL=http://localhost:9000/uploads  # for disk.URL()
package minio

import (
	"github.com/godx-jp/godx-platform-framework/storage/driver"
	"github.com/godx-jp/godx-platform-framework/storage/drivers/internal/s3core"
)

func init() {
	driver.Register(driver.DriverMinIO, s3core.NewConstructor(s3core.ProfileMinIO))
}

// Name is exported so callers can reference the driver name without
// hard-coding a string.
const Name = driver.DriverMinIO
