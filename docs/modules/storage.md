[← docs index](../README.md)

# Storage

Multi-disk file and object storage for Go services — Laravel `Storage` parity packaged as a framework module.

A single `Manager` holds one or more named `Disk` handles; each disk is backed by a driver chosen at deploy time (filesystem, in-memory, AWS S3, GCS, Azure Blob, MinIO). The application code never knows or imports a driver — backend swapping is an env-var change.

## At a glance

| Concept | Laravel | godx-platform-framework |
|---------|---------|-------------------------|
| Default disk name | `FILESYSTEM_DISK` | `STORAGE_DEFAULT_DISK` |
| List of disks | `config/filesystems.php` `disks` array | `STORAGE_DISKS` + per-disk `STORAGE_DISK_<NAME>_*` |
| Get a disk | `Storage::disk('s3')` | `mgr.Disk("s3")` |
| Default disk | `Storage::put(...)` | `mgr.Default().Put(...)` |
| Write | `Storage::put($path, $content)` | `disk.Put(ctx, path, content)` |
| Read | `Storage::get($path)` | `disk.Get(ctx, path)` |
| Streaming write | `Storage::writeStream($path, $stream)` | `disk.WriteStream(ctx, path, r)` |
| Streaming read | `Storage::readStream($path)` | `disk.ReadStream(ctx, path)` |
| Public URL | `Storage::url($path)` | `disk.URL(path)` |
| Temporary URL | `Storage::temporaryUrl($path, now()->addMins(5))` | `disk.TemporaryURL(ctx, path, 5*time.Minute)` |
| Visibility | `Storage::setVisibility($p, 'public')` | `disk.Put(ctx, p, c, storage.WithVisibility(driver.VisibilityPublic))` |

## Quick start

Zero env vars set — you get a single `local` disk rooted at `./storage`, private visibility:

```go
import (
    "context"
    "github.com/godx-jp/godx-platform-framework/framework"
    "github.com/godx-jp/godx-platform-framework/storage"
)

func main() {
    ctx := context.Background()
    app := framework.New("my-svc", "1.0.0").Use(storage.Module)
    if err := app.Init(ctx); err != nil { panic(err) }
    defer app.Shutdown(ctx)

    mgr, _ := storage.FromApp(app)
    disk, _ := mgr.Disk("local")

    _ = disk.Put(ctx, "hello.txt", []byte("world"))
    body, _ := disk.Get(ctx, "hello.txt")
    println(string(body)) // world
}
```

## Multiple disks via env

```bash
STORAGE_DEFAULT_DISK=uploads
STORAGE_DISKS=uploads,avatars,cache

STORAGE_DISK_UPLOADS_DRIVER=local
STORAGE_DISK_UPLOADS_ROOT=/var/data/uploads
STORAGE_DISK_UPLOADS_VISIBILITY=private

STORAGE_DISK_AVATARS_DRIVER=local
STORAGE_DISK_AVATARS_ROOT=/var/data/avatars
STORAGE_DISK_AVATARS_VISIBILITY=public
STORAGE_DISK_AVATARS_PUBLIC_URL=https://cdn.example.com/avatars

STORAGE_DISK_CACHE_DRIVER=memory
```

```go
mgr, _ := storage.FromApp(app)

avatars, _ := mgr.Disk("avatars")
_ = avatars.Put(ctx, "user-42.jpg", img,
    storage.WithContentType("image/jpeg"),
    storage.WithVisibility(driver.VisibilityPublic),
)
url, _ := avatars.URL("user-42.jpg")
// → "https://cdn.example.com/avatars/user-42.jpg"

cache, _ := mgr.Disk("cache")
_ = cache.Put(ctx, "session/abc", payload)
```

## Programmatic configuration

For tests or codebases that prefer wiring in code, skip the env loader:

```go
cfg := storage.Config{
    DefaultDisk: "local",
    Disks: map[string]storage.DiskConfig{
        "local": {Driver: "local", Root: t.TempDir()},
        "mem":   {Driver: "memory"},
    },
}
app := framework.New("test", "1.0.0").Use(storage.ModuleWithConfig(cfg))
```

Adding a disk after the primary module — useful for code-defined extras alongside env-defined ones:

```go
app := framework.New("svc", "1.0.0").
    Use(storage.Module).
    Use(storage.AddDisk("audit-archive", storage.DiskConfig{
        Driver:            driver.DriverS3,
        Bucket:            "audit-bucket",
        Region:            "ap-northeast-1",
        DefaultVisibility: driver.VisibilityPrivate,
    }))
```

`AddDisk` must come **after** `Module`; the framework returns a clear ordering error otherwise.

## The Disk API

| Method | Notes |
|--------|-------|
| `Put(ctx, path, content, opts...)` | Replaces; functional `PutOption` values |
| `Get(ctx, path) ([]byte, error)` | Returns `driver.ErrNotFound` when absent |
| `Exists(ctx, path) (bool, error)` | |
| `Missing(ctx, path) (bool, error)` | Convenience inverse of `Exists` |
| `Delete(ctx, path) error` | `driver.ErrNotFound` when absent |
| `Copy(ctx, src, dst) error` | Same disk; cross-disk via `Get` + `Put` on caller |
| `Move(ctx, src, dst) error` | Copy + Delete src |
| `Size(ctx, path) (int64, error)` | |
| `LastModified(ctx, path) (time.Time, error)` | |
| `Attributes(ctx, path) (driver.Attributes, error)` | Full metadata record |
| `Files(ctx, dir) ([]string, error)` | Immediate file children only (non-recursive) |
| `Directories(ctx, dir) ([]string, error)` | Immediate sub-directories only |
| `List(ctx, dir) ([]driver.Entry, error)` | Files + directories together |
| `ReadStream(ctx, path) (io.ReadCloser, error)` | For large objects |
| `WriteStream(ctx, path, r, opts...) error` | For large objects |
| `Append(ctx, path, content) error` | Round-trips through Get/Put; not atomic — log-style usage only |
| `Prepend(ctx, path, content) error` | Same caveat as `Append` |
| `URL(path) (string, error)` | Returns `driver.ErrNotSupported` on drivers without a public URL surface |
| `TemporaryURL(ctx, path, expires) (string, error)` | Returns `driver.ErrNotSupported` on drivers without signing |

### Write options

```go
disk.Put(ctx, "report.pdf", body,
    storage.WithContentType("application/pdf"),
    storage.WithVisibility(driver.VisibilityPrivate),
    storage.WithCacheControl("max-age=86400"),
    storage.WithMetadata(map[string]string{"author": "alice"}),
)
```

## Driver matrix

| Driver | Status | Registration | Notes |
|--------|--------|--------------|-------|
| `local` | stable | auto | Filesystem with `..` traversal guard; visibility maps to file mode (`0644` public / `0600` private) |
| `memory` | stable | auto | In-memory map; `Shutdown` clears all objects. Ideal for tests |
| `s3` | stub (v0.6.0) → full (v0.6.x) | opt-in (`_ "...drivers/s3"`) | AWS S3 |
| `gcs` | stub | opt-in (`_ "...drivers/gcs"`) | Google Cloud Storage |
| `azure` | stub | opt-in (`_ "...drivers/azure"`) | Azure Blob Storage |
| `minio` | stub | opt-in (`_ "...drivers/minio"`) | MinIO / S3-compatible (defaults `UsePathStyle=true`, requires `ENDPOINT`) |

**Light** drivers (`local`, `memory`) are auto-registered when you `import "...storage"`.

**Heavy** drivers carry cloud-SDK dependencies and are registered only when explicitly blank-imported, so binaries that only need `local` stay lean:

```go
import _ "github.com/godx-jp/godx-platform-framework/storage/drivers/s3"
```

Selecting a heavy driver without importing it fails at module init with a hint that names the missing import path.

## Error model

All driver errors are wrapped with stable sentinels — test with `errors.Is`:

```go
body, err := disk.Get(ctx, "missing.txt")
switch {
case errors.Is(err, driver.ErrNotFound):       // 404-style
case errors.Is(err, driver.ErrNotSupported):    // capability gap (e.g. local SignedURL)
case errors.Is(err, driver.ErrNotImplemented):  // heavy driver stub before v0.6.x
case err != nil:                                // unexpected
default:                                        // success
}
```

## Context propagation

`storage.ContextWithManager(ctx, mgr)` attaches a manager to a context for handlers that prefer pulling from `context.Context` over closures. `storage.FromContext(ctx)` retrieves it.

`storage.FromApp(app)` is the canonical way to retrieve the manager built by `storage.Module`.

## Lifecycle

`storage.Module` registers `Manager.Shutdown` via `app.OnShutdown`. Every disk's driver is asked to release resources in turn; errors are joined so a misbehaving driver does not block the rest. Shutdown is idempotent.

## Security: path traversal

The `local` driver rejects keys containing `..` segments **before** path cleaning to defeat the `Clean("/../x") == "/x"` foot-gun. Combined with a sanity check that the resulting absolute path remains under the disk root, this matches the defence-in-depth posture of league/flysystem. Absolute keys and Windows-style backslashes are normalised; empty keys are rejected.

## Reference

- Layout convention shared with every other framework module: [DRIVER_PATTERN](../DRIVER_PATTERN.md)
- Every env var in one place: [CONFIGURATION § Storage](../CONFIGURATION.md#storage--common)
- Architecture & roadmap: [ARCHITECTURE](../ARCHITECTURE.md)
