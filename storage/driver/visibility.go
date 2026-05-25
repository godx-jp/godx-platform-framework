package driver

// Visibility is the access policy applied to an object on a disk.
//
// Mirrors Laravel's two-value `visibility` setting. Object stores like
// S3 translate VisibilityPublic to a public-read ACL; the local driver
// translates it to a world-readable file mode.
type Visibility string

const (
	// VisibilityPrivate restricts access to authenticated principals
	// (object stores) or the file owner (local driver). This is the
	// safe default for arbitrary user uploads.
	VisibilityPrivate Visibility = "private"

	// VisibilityPublic grants unauthenticated read access. Use only
	// for genuinely public assets (CDN images, public documents).
	VisibilityPublic Visibility = "public"
)

// IsValid reports whether v is a recognised visibility value. Empty is
// not valid — callers must resolve "unset" to the driver default
// before validation.
func (v Visibility) IsValid() bool {
	switch v {
	case VisibilityPrivate, VisibilityPublic:
		return true
	default:
		return false
	}
}

// String returns the canonical string form for env serialisation.
func (v Visibility) String() string { return string(v) }
