// Package local implements a storage.Driver that stores objects on the
// local filesystem rooted at a configurable directory.
//
// The driver is "light" — it depends only on the standard library and
// is auto-registered when the parent storage package is imported. No
// blank import is required.
package local

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/godx-jp/godx-platform-framework/storage/driver"
)

func init() {
	driver.Register(driver.DriverLocal, construct)
}

// Name is exported so callers can reference the driver name without
// hard-coding a string.
const Name = driver.DriverLocal

// File mode constants applied to writes based on visibility.
//
// Mirrors Laravel's `flysystem-local` defaults:
//   - private: file 0600, dir 0700
//   - public : file 0644, dir 0755
const (
	publicFileMode  os.FileMode = 0o644
	publicDirMode   os.FileMode = 0o755
	privateFileMode os.FileMode = 0o600
	privateDirMode  os.FileMode = 0o700
)

func construct(_ context.Context, s driver.Spec) (driver.Driver, error) {
	if strings.TrimSpace(s.Root) == "" {
		return nil, fmt.Errorf("local: root path is required")
	}
	root, err := filepath.Abs(s.Root)
	if err != nil {
		return nil, fmt.Errorf("local: resolve root %q: %w", s.Root, err)
	}
	if err := os.MkdirAll(root, defaultDirMode(s.DefaultVisibility)); err != nil {
		return nil, fmt.Errorf("local: create root %q: %w", root, err)
	}
	return &impl{
		root:         root,
		defaultVis:   defaultVisibility(s.DefaultVisibility),
		publicURLBase: strings.TrimRight(s.PublicURL, "/"),
	}, nil
}

type impl struct {
	root          string
	defaultVis    driver.Visibility
	publicURLBase string

	shutdownOnce sync.Once
}

func defaultVisibility(v driver.Visibility) driver.Visibility {
	if v.IsValid() {
		return v
	}
	return driver.VisibilityPrivate
}

func defaultDirMode(v driver.Visibility) os.FileMode {
	if defaultVisibility(v) == driver.VisibilityPublic {
		return publicDirMode
	}
	return privateDirMode
}

func fileMode(v driver.Visibility) os.FileMode {
	if v == driver.VisibilityPublic {
		return publicFileMode
	}
	return privateFileMode
}

func dirMode(v driver.Visibility) os.FileMode {
	if v == driver.VisibilityPublic {
		return publicDirMode
	}
	return privateDirMode
}

// safePath joins key onto the driver root and verifies the resulting
// absolute path is still under the root. Defeats absolute-path inputs
// and `../` traversal — `..` segments are rejected outright rather
// than silently normalised away by path.Clean, matching the
// defence-in-depth posture of league/flysystem.
func (d *impl) safePath(key string) (string, error) {
	if strings.TrimSpace(key) == "" {
		return "", fmt.Errorf("local: empty key")
	}
	raw := strings.ReplaceAll(key, `\`, "/")
	for _, seg := range strings.Split(raw, "/") {
		if seg == ".." {
			return "", fmt.Errorf("local: path %q escapes root (contains ..)", key)
		}
	}
	cleaned := path.Clean("/" + raw)
	if cleaned == "/" {
		return "", fmt.Errorf("local: empty key after clean")
	}
	abs := filepath.Join(d.root, filepath.FromSlash(cleaned))
	rel, err := filepath.Rel(d.root, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("local: path %q escapes root", key)
	}
	return abs, nil
}

func (d *impl) NewReader(_ context.Context, key string) (io.ReadCloser, error) {
	p, err := d.safePath(key)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(p)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%w: %s", driver.ErrNotFound, key)
	}
	if err != nil {
		return nil, fmt.Errorf("local: open %q: %w", key, err)
	}
	return f, nil
}

func (d *impl) NewWriter(_ context.Context, key string, opts driver.WriteOptions) (io.WriteCloser, error) {
	p, err := d.safePath(key)
	if err != nil {
		return nil, err
	}
	vis := opts.Visibility
	if !vis.IsValid() {
		vis = d.defaultVis
	}
	if err := os.MkdirAll(filepath.Dir(p), dirMode(vis)); err != nil {
		return nil, fmt.Errorf("local: mkdir for %q: %w", key, err)
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fileMode(vis))
	if err != nil {
		return nil, fmt.Errorf("local: open writer %q: %w", key, err)
	}
	return f, nil
}

func (d *impl) Delete(_ context.Context, key string) error {
	p, err := d.safePath(key)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("%w: %s", driver.ErrNotFound, key)
		}
		return fmt.Errorf("local: delete %q: %w", key, err)
	}
	return nil
}

func (d *impl) Exists(_ context.Context, key string) (bool, error) {
	p, err := d.safePath(key)
	if err != nil {
		return false, err
	}
	info, err := os.Stat(p)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("local: stat %q: %w", key, err)
	}
	// Treat directories as "not an object" for Laravel parity.
	return !info.IsDir(), nil
}

func (d *impl) Attributes(_ context.Context, key string) (driver.Attributes, error) {
	p, err := d.safePath(key)
	if err != nil {
		return driver.Attributes{}, err
	}
	info, err := os.Stat(p)
	if errors.Is(err, fs.ErrNotExist) {
		return driver.Attributes{}, fmt.Errorf("%w: %s", driver.ErrNotFound, key)
	}
	if err != nil {
		return driver.Attributes{}, fmt.Errorf("local: stat %q: %w", key, err)
	}
	return driver.Attributes{
		Size:         info.Size(),
		LastModified: info.ModTime(),
		Visibility:   visibilityFromMode(info.Mode().Perm()),
	}, nil
}

func visibilityFromMode(mode os.FileMode) driver.Visibility {
	// Anything readable by "others" counts as public.
	if mode&0o004 != 0 {
		return driver.VisibilityPublic
	}
	return driver.VisibilityPrivate
}

func (d *impl) List(_ context.Context, prefix string) ([]driver.Entry, error) {
	dir, err := d.safePath(prefix)
	if err != nil {
		// Allow listing the root via empty/"/" prefix.
		if strings.TrimSpace(prefix) == "" || prefix == "/" {
			dir = d.root
		} else {
			return nil, err
		}
	}
	info, err := os.Stat(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("local: stat dir %q: %w", prefix, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("local: %q is not a directory", prefix)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("local: read dir %q: %w", prefix, err)
	}
	out := make([]driver.Entry, 0, len(entries))
	for _, e := range entries {
		rel, _ := filepath.Rel(d.root, filepath.Join(dir, e.Name()))
		key := filepath.ToSlash(rel)
		entry := driver.Entry{Key: key, IsDir: e.IsDir()}
		if !e.IsDir() {
			if fi, ierr := e.Info(); ierr == nil {
				entry.Size = fi.Size()
				entry.LastModified = fi.ModTime()
			}
		}
		out = append(out, entry)
	}
	return out, nil
}

func (d *impl) URL(key string) (string, error) {
	if d.publicURLBase == "" {
		return "", fmt.Errorf("%w: configure Spec.PublicURL to enable URL()", driver.ErrNotSupported)
	}
	// Encode each path segment but keep slashes.
	parts := strings.Split(strings.TrimLeft(key, "/"), "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return d.publicURLBase + "/" + strings.Join(parts, "/"), nil
}

func (d *impl) SignedURL(_ context.Context, _ string, _ time.Duration) (string, error) {
	// The local filesystem has no signing concept. A real implementation
	// would require a separate signing service, which is out of scope.
	return "", fmt.Errorf("%w: local driver cannot sign URLs", driver.ErrNotSupported)
}

func (d *impl) Shutdown(_ context.Context) error {
	d.shutdownOnce.Do(func() {})
	return nil
}
