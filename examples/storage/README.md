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

## Heavy drivers

Add the matching blank import to `main.go` and configure with env vars.

### AWS S3 (stable v0.6.1)

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

### MinIO / S3-compatible (stable v0.6.1)

```go
import _ "github.com/godx-jp/godx-platform-framework/storage/drivers/minio"
```

Boot MinIO once:

```bash
docker run --rm -d --name minio-dev -p 9000:9000 -p 9001:9001 \
    -e MINIO_ROOT_USER=minioadmin -e MINIO_ROOT_PASSWORD=minioadmin \
    quay.io/minio/minio server /data --console-address :9001
docker exec minio-dev mc alias set local http://127.0.0.1:9000 minioadmin minioadmin
docker exec minio-dev mc mb local/uploads
```

```bash
STORAGE_DEFAULT_DISK=mc \
STORAGE_DISKS=mc \
STORAGE_DISK_MC_DRIVER=minio \
STORAGE_DISK_MC_BUCKET=uploads \
STORAGE_DISK_MC_ENDPOINT=http://localhost:9000 \
STORAGE_DISK_MC_ACCESS_KEY=minioadmin \
STORAGE_DISK_MC_SECRET_KEY=minioadmin \
go run ./examples/storage
```

### GCS / Azure (stubs)

These still return `driver.ErrNotImplemented` through v0.6.x until their full implementations land.

## See also

- [docs/modules/storage](../../docs/modules/storage.md)
- [docs/CONFIGURATION § Storage](../../docs/CONFIGURATION.md#storage--common)
- [docs/DRIVER_PATTERN](../../docs/DRIVER_PATTERN.md)
