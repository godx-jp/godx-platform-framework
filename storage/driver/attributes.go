package driver

import "time"

// WriteOptions controls a single write operation. Drivers fall back to
// sensible defaults for unset fields (driver-specific Visibility,
// auto-detected ContentType, nil Metadata).
type WriteOptions struct {
	// ContentType is the MIME type of the object (e.g.
	// "image/jpeg"). Drivers may auto-detect when empty.
	ContentType string

	// Visibility controls the access policy applied to the object on
	// write. Empty means "use the driver's DefaultVisibility".
	Visibility Visibility

	// Metadata is user-defined key/value metadata attached to the
	// object. Drivers without metadata support silently drop it.
	Metadata map[string]string

	// CacheControl maps to the Cache-Control header for object stores.
	CacheControl string
}

// Attributes describes an object stored on a disk.
type Attributes struct {
	// Size is the object size in bytes.
	Size int64

	// LastModified is the last-modified timestamp reported by the
	// backend.
	LastModified time.Time

	// ContentType is the MIME type recorded on write.
	ContentType string

	// Visibility is the access policy currently in effect.
	Visibility Visibility

	// Metadata is the user-defined key/value metadata.
	Metadata map[string]string

	// ETag is a content-derived identifier (object stores). Empty for
	// drivers without an ETag concept.
	ETag string
}

// Entry is one element returned by List.
type Entry struct {
	// Key is the full key relative to the disk root.
	Key string

	// IsDir reports whether this entry represents a directory (or, for
	// object stores, a common prefix).
	IsDir bool

	// Size is the entry size in bytes; zero for directories.
	Size int64

	// LastModified is the last-modified timestamp; zero value for
	// directories.
	LastModified time.Time
}
