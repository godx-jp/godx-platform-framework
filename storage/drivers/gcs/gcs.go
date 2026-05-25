// Package gcs wires a Google Cloud Storage-backed storage driver into
// the registry.
//
// HEAVY driver — opt in with a blank import:
//
//	import _ "github.com/godx-jp/godx-platform-framework/storage/drivers/gcs"
//
// Full implementation lands in a v0.6.x patch release. The current
// constructor returns driver.ErrNotImplemented so misconfigurations
// surface at startup.
package gcs

import (
	"context"
	"fmt"

	"github.com/godx-jp/godx-platform-framework/storage/driver"
)

func init() {
	driver.Register(driver.DriverGCS, construct)
}

const Name = driver.DriverGCS

func construct(_ context.Context, _ driver.Spec) (driver.Driver, error) {
	return nil, fmt.Errorf(
		"%w: storage/drivers/gcs — full implementation landing in v0.6.x",
		driver.ErrNotImplemented,
	)
}
