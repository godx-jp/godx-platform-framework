// Package s3 wires an AWS S3-backed storage driver into the registry.
//
// This is a HEAVY driver — it depends on the AWS SDK for Go v2 and is
// not part of the framework's auto-registered driver set. Consumers
// opt in with a blank import so binaries that do not need S3 avoid the
// SDK dependency entirely:
//
//	import _ "github.com/godx-jp/godx-platform-framework/storage/drivers/s3"
//
// The full S3 implementation lands in a v0.6.x patch release. The
// current constructor returns driver.ErrNotImplemented so callers fail
// fast at startup rather than at first write, and they get a clear
// error message pointing at the dependency that must be added.
package s3

import (
	"context"
	"fmt"

	"github.com/godx-jp/godx-platform-framework/storage/driver"
)

func init() {
	driver.Register(driver.DriverS3, construct)
}

const Name = driver.DriverS3

func construct(_ context.Context, _ driver.Spec) (driver.Driver, error) {
	return nil, fmt.Errorf(
		"%w: storage/drivers/s3 — full implementation landing in v0.6.x. "+
			"Track progress in CHANGELOG; until then use the local or memory driver",
		driver.ErrNotImplemented,
	)
}
