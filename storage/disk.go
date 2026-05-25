package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/godx-jp/godx-platform-framework/storage/driver"
)

// Disk is the user-facing handle for a single configured storage
// backend. It wraps a driver.Driver with Laravel-style ergonomic
// methods so application code never has to think about WriteOptions or
// stream lifecycle for simple cases.
//
// All methods are safe for concurrent use by multiple goroutines —
// concurrency is delegated to the underlying driver, which is required
// to be goroutine-safe.
type Disk struct {
	name   string
	driver driver.Driver
	config DiskConfig
}

// NewDiskFromDriver wraps an already-constructed driver as a Disk.
// Useful when you need a Disk handle without going through the
// config-driven module (custom drivers, test fixtures).
func NewDiskFromDriver(name string, drv driver.Driver, cfg DiskConfig) *Disk {
	return &Disk{name: name, driver: drv, config: cfg}
}

// Name returns the configured name of the disk (e.g. "uploads", "s3").
func (d *Disk) Name() string { return d.name }

// Driver returns the canonical name of the backing driver
// (e.g. "local", "s3").
func (d *Disk) Driver() string { return d.config.Driver }

// PutOption is a functional option for write operations.
type PutOption func(*driver.WriteOptions)

// WithContentType sets the MIME type for the next write.
func WithContentType(ct string) PutOption {
	return func(o *driver.WriteOptions) { o.ContentType = ct }
}

// WithVisibility overrides the disk default visibility for this write.
func WithVisibility(v driver.Visibility) PutOption {
	return func(o *driver.WriteOptions) { o.Visibility = v }
}

// WithMetadata attaches user-defined metadata to the object. Drivers
// without metadata support silently ignore it.
func WithMetadata(m map[string]string) PutOption {
	return func(o *driver.WriteOptions) {
		if o.Metadata == nil {
			o.Metadata = map[string]string{}
		}
		for k, v := range m {
			o.Metadata[k] = v
		}
	}
}

// WithCacheControl sets the Cache-Control header for object stores.
func WithCacheControl(cc string) PutOption {
	return func(o *driver.WriteOptions) { o.CacheControl = cc }
}

func buildOpts(opts []PutOption) driver.WriteOptions {
	var o driver.WriteOptions
	for _, fn := range opts {
		fn(&o)
	}
	return o
}

// Put writes content to path, replacing any existing object.
// Equivalent to Laravel's Storage::put($path, $content).
func (d *Disk) Put(ctx context.Context, path string, content []byte, opts ...PutOption) error {
	w, err := d.driver.NewWriter(ctx, path, buildOpts(opts))
	if err != nil {
		return err
	}
	if _, err := w.Write(content); err != nil {
		_ = w.Close()
		return fmt.Errorf("storage: write %q: %w", path, err)
	}
	return w.Close()
}

// Get returns the entire object at path. Returns driver.ErrNotFound
// when path does not exist.
func (d *Disk) Get(ctx context.Context, path string) ([]byte, error) {
	r, err := d.driver.NewReader(ctx, path)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

// Exists reports whether path is present on the disk.
func (d *Disk) Exists(ctx context.Context, path string) (bool, error) {
	return d.driver.Exists(ctx, path)
}

// Missing is the convenience inverse of Exists.
func (d *Disk) Missing(ctx context.Context, path string) (bool, error) {
	ok, err := d.driver.Exists(ctx, path)
	return !ok, err
}

// Delete removes path. Returns driver.ErrNotFound when absent.
func (d *Disk) Delete(ctx context.Context, path string) error {
	return d.driver.Delete(ctx, path)
}

// Size returns the size of the object at path in bytes.
func (d *Disk) Size(ctx context.Context, path string) (int64, error) {
	a, err := d.driver.Attributes(ctx, path)
	if err != nil {
		return 0, err
	}
	return a.Size, nil
}

// LastModified returns the last-modified timestamp of path.
func (d *Disk) LastModified(ctx context.Context, path string) (time.Time, error) {
	a, err := d.driver.Attributes(ctx, path)
	if err != nil {
		return time.Time{}, err
	}
	return a.LastModified, nil
}

// Attributes returns the full metadata record for path.
func (d *Disk) Attributes(ctx context.Context, path string) (driver.Attributes, error) {
	return d.driver.Attributes(ctx, path)
}

// Files lists the immediate file children of dir (non-recursive),
// returning their keys.
func (d *Disk) Files(ctx context.Context, dir string) ([]string, error) {
	entries, err := d.driver.List(ctx, dir)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir {
			out = append(out, e.Key)
		}
	}
	return out, nil
}

// Directories lists the immediate sub-directory children of dir
// (non-recursive), returning their keys.
func (d *Disk) Directories(ctx context.Context, dir string) ([]string, error) {
	entries, err := d.driver.List(ctx, dir)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir {
			out = append(out, e.Key)
		}
	}
	return out, nil
}

// List returns the immediate children of dir as raw Entry records.
func (d *Disk) List(ctx context.Context, dir string) ([]driver.Entry, error) {
	return d.driver.List(ctx, dir)
}

// ReadStream returns a streaming reader for path. Caller must Close.
// Use this instead of Get for large objects.
func (d *Disk) ReadStream(ctx context.Context, path string) (io.ReadCloser, error) {
	return d.driver.NewReader(ctx, path)
}

// WriteStream copies r into path. Use this instead of Put for large
// objects.
func (d *Disk) WriteStream(ctx context.Context, path string, r io.Reader, opts ...PutOption) error {
	w, err := d.driver.NewWriter(ctx, path, buildOpts(opts))
	if err != nil {
		return err
	}
	if _, err := io.Copy(w, r); err != nil {
		_ = w.Close()
		return fmt.Errorf("storage: writestream %q: %w", path, err)
	}
	return w.Close()
}

// Copy duplicates the object at src to dst on the same disk. Cross-disk
// copy must be done by the caller (Get on src disk, Put on dst disk).
func (d *Disk) Copy(ctx context.Context, src, dst string) error {
	r, err := d.driver.NewReader(ctx, src)
	if err != nil {
		return err
	}
	defer r.Close()
	w, err := d.driver.NewWriter(ctx, dst, driver.WriteOptions{})
	if err != nil {
		return err
	}
	if _, err := io.Copy(w, r); err != nil {
		_ = w.Close()
		return fmt.Errorf("storage: copy %q->%q: %w", src, dst, err)
	}
	return w.Close()
}

// Move renames src to dst (Copy + Delete on src).
func (d *Disk) Move(ctx context.Context, src, dst string) error {
	if err := d.Copy(ctx, src, dst); err != nil {
		return err
	}
	if err := d.driver.Delete(ctx, src); err != nil && !errors.Is(err, driver.ErrNotFound) {
		return fmt.Errorf("storage: move %q->%q: delete src: %w", src, dst, err)
	}
	return nil
}

// URL returns a stable public URL for path. Drivers without a notion of
// a public URL return driver.ErrNotSupported.
func (d *Disk) URL(path string) (string, error) {
	return d.driver.URL(path)
}

// TemporaryURL returns a signed URL granting read access for expires
// duration. Drivers without signed-URL support return
// driver.ErrNotSupported.
func (d *Disk) TemporaryURL(ctx context.Context, path string, expires time.Duration) (string, error) {
	return d.driver.SignedURL(ctx, path, expires)
}

// Prepend writes content to the start of path, preserving any existing
// content. Convenient for log-style append-front patterns.
func (d *Disk) Prepend(ctx context.Context, path string, content []byte) error {
	existing, err := d.Get(ctx, path)
	if err != nil && !errors.Is(err, driver.ErrNotFound) {
		return err
	}
	return d.Put(ctx, path, append(content, existing...))
}

// Append writes content to the end of path, preserving any existing
// content.
func (d *Disk) Append(ctx context.Context, path string, content []byte) error {
	existing, err := d.Get(ctx, path)
	if err != nil && !errors.Is(err, driver.ErrNotFound) {
		return err
	}
	var buf bytes.Buffer
	buf.Write(existing)
	buf.Write(content)
	return d.Put(ctx, path, buf.Bytes())
}
