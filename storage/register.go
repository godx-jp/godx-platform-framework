package storage

// Blank imports for LIGHT drivers — auto-registered via the parent
// package import. Heavy drivers (s3, gcs, azure, minio) must be
// imported explicitly by the consumer to keep binaries lean. See
// docs/DRIVER_PATTERN.md.
import (
	_ "github.com/godx-jp/godx-platform-framework/storage/drivers/local"
	_ "github.com/godx-jp/godx-platform-framework/storage/drivers/memory"
)
