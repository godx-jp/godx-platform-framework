// Package storage exposes a multi-disk file/object storage abstraction
// modelled on Laravel's Storage facade. A single Manager holds one or
// more named Disks; each Disk is backed by a driver chosen at
// configuration time (local filesystem, in-memory, AWS S3, Google Cloud
// Storage, Azure Blob, MinIO, …).
//
// # Layout
//
//	storage/                  ← this package — Manager, Disk, Module, env loaders
//	storage/driver/           ← public driver contract (Driver, Spec, registry, Visibility)
//	storage/drivers/<name>/   ← per-implementation packages
//
// # Driver weight
//
// "Light" drivers (local, memory) depend only on the standard library
// and are auto-registered via the import side effects in register.go.
//
// "Heavy" drivers (s3, gcs, azure, minio) carry cloud-SDK dependencies
// and require an explicit blank import to register so binaries that do
// not need them stay lean:
//
//	import _ "github.com/godx-jp/godx-platform-framework/storage/drivers/s3"
//
// See docs/DRIVER_PATTERN.md for the cross-module convention shared
// with the observability module (and the storage/cache/queue modules
// that follow).
//
// # Usage
//
//	app := framework.New("svc", "1.0.0").Use(storage.Module)
//	if err := app.Init(ctx); err != nil { return err }
//
//	mgr, _ := storage.FromApp(app)
//	disk, _ := mgr.Disk("local")
//	_ = disk.Put(ctx, "hello.txt", []byte("world"))
//	body, _ := disk.Get(ctx, "hello.txt")
//
// Default behaviour with zero env vars set: one disk named "local"
// rooted at "./storage", VisibilityPrivate.
package storage
