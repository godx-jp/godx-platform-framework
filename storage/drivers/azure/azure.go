// Package azure wires an Azure Blob Storage-backed storage driver into
// the registry.
//
// HEAVY driver — opt in with a blank import:
//
//	import _ "github.com/godx-jp/godx-platform-framework/storage/drivers/azure"
//
// Full implementation lands in a v0.6.x patch release.
package azure

import (
	"context"
	"fmt"

	"github.com/godx-jp/godx-platform-framework/storage/driver"
)

func init() {
	driver.Register(driver.DriverAzure, construct)
}

const Name = driver.DriverAzure

func construct(_ context.Context, _ driver.Spec) (driver.Driver, error) {
	return nil, fmt.Errorf(
		"%w: storage/drivers/azure — full implementation landing in v0.6.x",
		driver.ErrNotImplemented,
	)
}
