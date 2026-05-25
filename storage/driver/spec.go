package driver

// Spec is the uniform configuration input passed to every driver
// constructor. Fields not used by a given driver are simply ignored, so
// new drivers can rely on a single struct rather than each defining
// their own.
//
// Spec mirrors observability/driver.Spec — see docs/DRIVER_PATTERN.md.
type Spec struct {
	// Name is the driver name (e.g. "local", "s3"). The registry sets
	// this before invoking the constructor; drivers can use it for
	// logging or error annotation.
	Name string

	// Disk is the logical disk name from configuration (e.g. "uploads",
	// "avatars"). Useful for error annotation; safe to ignore.
	Disk string

	// ── Local-filesystem driver ──────────────────────────────────────
	// Root is the absolute (or working-directory-relative) filesystem
	// path under which the local driver stores objects.
	Root string

	// DefaultVisibility is the visibility applied to writes that do not
	// specify one explicitly. Defaults to VisibilityPrivate.
	DefaultVisibility Visibility

	// ── Object-store drivers (s3, gcs, azure, minio) ─────────────────
	// Bucket is the bucket / container name.
	Bucket string

	// Region is the cloud region (e.g. "ap-northeast-1").
	Region string

	// Endpoint is a custom endpoint URL. Required for MinIO and other
	// S3-compatible stores; optional for AWS S3 (defaults to AWS).
	Endpoint string

	// UsePathStyle selects S3 path-style addressing
	// (`<endpoint>/<bucket>/<key>`) instead of virtual-hosted style
	// (`<bucket>.<endpoint>/<key>`). Required for most S3-compatible
	// stores including MinIO.
	UsePathStyle bool

	// AccessKey / SecretKey are explicit credentials. Most cloud SDKs
	// pick up credentials from the environment / metadata service
	// automatically, so leaving these empty is usually correct.
	AccessKey string
	SecretKey string

	// SessionToken is the optional AWS STS session token.
	SessionToken string

	// ── URL generation ───────────────────────────────────────────────
	// PublicURL is the base URL prefixed to keys when Disk.URL is
	// called. For example PublicURL="https://cdn.example.com" causes
	// disk.URL("avatars/1.jpg") to return
	// "https://cdn.example.com/avatars/1.jpg".
	PublicURL string

	// ── Free-form extension ──────────────────────────────────────────
	// Extra holds driver-specific keys not modelled above. Drivers
	// document the keys they recognise; unknown keys are ignored.
	Extra map[string]string
}
