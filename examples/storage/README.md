# Storage example

End-to-end demo of the `storage` module — Laravel-style multi-disk file/object storage.

## Run

Zero env vars — uses the default `local` disk rooted at `./storage`:

```bash
go run ./examples/storage
```

Multiple disks via env:

```bash
STORAGE_DEFAULT_DISK=uploads \
STORAGE_DISKS=uploads,cache \
STORAGE_DISK_UPLOADS_DRIVER=local \
STORAGE_DISK_UPLOADS_ROOT=/tmp/example-uploads \
STORAGE_DISK_UPLOADS_VISIBILITY=public \
STORAGE_DISK_UPLOADS_PUBLIC_URL=https://cdn.example.com \
STORAGE_DISK_CACHE_DRIVER=memory \
go run ./examples/storage
```

## Heavy drivers (S3 / GCS / Azure / MinIO)

Add the matching blank import to `main.go` and configure with env vars. Through v0.6.0 the heavy drivers are stubs that return `driver.ErrNotImplemented`; the full implementations land in v0.6.x patches.

```go
import _ "github.com/godx-jp/godx-platform-framework/storage/drivers/s3"
```

```bash
STORAGE_DEFAULT_DISK=s3 \
STORAGE_DISKS=s3 \
STORAGE_DISK_S3_DRIVER=s3 \
STORAGE_DISK_S3_BUCKET=my-bucket \
STORAGE_DISK_S3_REGION=ap-northeast-1 \
AWS_PROFILE=dev \
go run ./examples/storage
```

## See also

- [docs/modules/storage](../../docs/modules/storage.md)
- [docs/CONFIGURATION § Storage](../../docs/CONFIGURATION.md#storage--common)
- [docs/DRIVER_PATTERN](../../docs/DRIVER_PATTERN.md)
