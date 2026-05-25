// Package s3 wires an AWS S3-backed storage driver into the registry.
//
// HEAVY driver — depends on github.com/aws/aws-sdk-go-v2 and the S3
// service module. It does NOT auto-register; consumers opt in with a
// blank import so binaries that do not need S3 avoid the SDK
// dependency entirely:
//
//	import _ "github.com/godx-jp/godx-platform-framework/storage/drivers/s3"
//
// Configuration (see docs/CONFIGURATION.md § Storage):
//
//	STORAGE_DISK_<NAME>_DRIVER=s3
//	STORAGE_DISK_<NAME>_BUCKET=my-bucket
//	STORAGE_DISK_<NAME>_REGION=ap-northeast-1
//	# Credentials usually come from the default AWS chain
//	# (env, ~/.aws/credentials, IRSA, EC2 IMDS). Override only
//	# when you have a static key:
//	# STORAGE_DISK_<NAME>_ACCESS_KEY=AKIA...
//	# STORAGE_DISK_<NAME>_SECRET_KEY=...
//	# STORAGE_DISK_<NAME>_PUBLIC_URL=https://cdn.example.com  # for disk.URL()
//
// The driver supports the full Laravel storage API on top of S3:
// Put/Get/Exists/Delete/Copy/Move/Size/LastModified/Files/Directories
// /ReadStream/WriteStream/URL/TemporaryURL. Multipart uploads are
// handled automatically via the official feature/s3/manager Uploader,
// so writes of arbitrary size are streamed efficiently.
package s3

import (
	"github.com/godx-jp/godx-platform-framework/storage/driver"
	"github.com/godx-jp/godx-platform-framework/storage/drivers/internal/s3core"
)

func init() {
	driver.Register(driver.DriverS3, s3core.NewConstructor(s3core.ProfileAWS))
}

// Name is exported so callers can reference the driver name without
// hard-coding a string.
const Name = driver.DriverS3
