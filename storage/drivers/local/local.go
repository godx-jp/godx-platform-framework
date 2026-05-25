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
		root:          root,
		defaultVis:    defaultVisibility(s.DefaultVisibility),
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

// relKey validates key and returns a slash-relative path suitable for
// use with *os.Root (which is rooted at d.root). It rejects absolute-path
// inputs and `..` traversal lexically as a first line of defence; the
// authoritative protection against symlink/`..` escape is os.Root, which
// refuses any operation that would resolve outside the root at the OS
// level (see the rooted* helpers below).
func (d *impl) relKey(key string) (string, error) {
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
	// Drop the leading slash so the result is relative to the root.
	rel := strings.TrimPrefix(cleaned, "/")
	return filepath.FromSlash(rel), nil
}

// openRoot opens an *os.Root anchored at the driver root for a single
// operation. os.Root refuses any path that traverses out of the root via
// symlinks or `..`, atomically and at the OS level, which the previous
// purely-lexical safePath could not do. Opening per-op (rather than
// caching one *os.Root) avoids fd-lifecycle bugs and keeps Shutdown
// trivial; the cost is one openat per call, which is negligible for a
// local-disk driver.
func (d *impl) openRoot() (*os.Root, error) {
	root, err := os.OpenRoot(d.root)
	if err != nil {
		return nil, fmt.Errorf("local: open root %q: %w", d.root, err)
	}
	return root, nil
}

// rootedMkdirAll mirrors os.MkdirAll for the directory components of a
// rel path, but every step goes through *os.Root so no component may be
// a symlink escaping the root. dir is the slash/OS-relative directory.
func rootedMkdirAll(root *os.Root, dir string, mode os.FileMode) error {
	dir = filepath.Clean(dir)
	if dir == "." || dir == string(filepath.Separator) {
		return nil
	}
	// Build the list of ancestors from shallowest to deepest.
	var parts []string
	for p := dir; ; {
		parts = append([]string{p}, parts...)
		parent := filepath.Dir(p)
		if parent == p {
			break
		}
		p = parent
	}
	for _, p := range parts {
		if p == "." || p == string(filepath.Separator) {
			continue
		}
		if err := root.Mkdir(p, mode); err != nil && !errors.Is(err, fs.ErrExist) {
			return err
		}
	}
	return nil
}

// rootFile pairs an open file with the *os.Root it was opened through so
// closing the file also releases the root for per-op usage.
type rootFile struct {
	*os.File
	root *os.Root
}

func (rf *rootFile) Close() error {
	ferr := rf.File.Close()
	rerr := rf.root.Close()
	if ferr != nil {
		return ferr
	}
	return rerr
}

func (d *impl) NewReader(_ context.Context, key string) (io.ReadCloser, error) {
	rel, err := d.relKey(key)
	if err != nil {
		return nil, err
	}
	root, err := d.openRoot()
	if err != nil {
		return nil, err
	}
	// os.Root.Open refuses symlink/`..` traversal out of the root.
	f, err := root.Open(rel)
	if errors.Is(err, fs.ErrNotExist) {
		_ = root.Close()
		return nil, fmt.Errorf("%w: %s", driver.ErrNotFound, key)
	}
	if err != nil {
		_ = root.Close()
		return nil, fmt.Errorf("local: open %q: %w", key, err)
	}
	return &rootFile{File: f, root: root}, nil
}

func (d *impl) NewWriter(_ context.Context, key string, opts driver.WriteOptions) (io.WriteCloser, error) {
	rel, err := d.relKey(key)
	if err != nil {
		return nil, err
	}
	vis := opts.Visibility
	if !vis.IsValid() {
		vis = d.defaultVis
	}
	root, err := d.openRoot()
	if err != nil {
		return nil, err
	}
	if dir := filepath.Dir(rel); dir != "." {
		if err := rootedMkdirAll(root, dir, dirMode(vis)); err != nil {
			_ = root.Close()
			return nil, fmt.Errorf("local: mkdir for %q: %w", key, err)
		}
	}
	// os.Root.OpenFile refuses symlink/`..` traversal out of the root.
	f, err := root.OpenFile(rel, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fileMode(vis))
	if err != nil {
		_ = root.Close()
		return nil, fmt.Errorf("local: open writer %q: %w", key, err)
	}
	// os.Root.OpenFile applies the requested perms only on creation and is
	// still subject to umask; Chmod afterwards guarantees the documented
	// 0600 (private) / 0644 (public) modes regardless of umask.
	if err := f.Chmod(fileMode(vis)); err != nil {
		_ = f.Close()
		_ = root.Close()
		return nil, fmt.Errorf("local: chmod %q: %w", key, err)
	}
	return &rootFile{File: f, root: root}, nil
}

func (d *impl) Delete(_ context.Context, key string) error {
	rel, err := d.relKey(key)
	if err != nil {
		return err
	}
	root, err := d.openRoot()
	if err != nil {
		return err
	}
	defer root.Close()
	// os.Root.Remove refuses symlink/`..` traversal out of the root and
	// will not follow a symlink to delete its target.
	if err := root.Remove(rel); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("%w: %s", driver.ErrNotFound, key)
		}
		return fmt.Errorf("local: delete %q: %w", key, err)
	}
	return nil
}

func (d *impl) Exists(_ context.Context, key string) (bool, error) {
	rel, err := d.relKey(key)
	if err != nil {
		return false, err
	}
	root, err := d.openRoot()
	if err != nil {
		return false, err
	}
	defer root.Close()
	info, err := root.Stat(rel)
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
	rel, err := d.relKey(key)
	if err != nil {
		return driver.Attributes{}, err
	}
	root, err := d.openRoot()
	if err != nil {
		return driver.Attributes{}, err
	}
	defer root.Close()
	info, err := root.Stat(rel)
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
	// relDir is "." for the root, otherwise the rooted relative directory.
	relDir := "."
	if strings.TrimSpace(prefix) != "" && prefix != "/" {
		r, err := d.relKey(prefix)
		if err != nil {
			return nil, err
		}
		relDir = r
	}
	root, err := d.openRoot()
	if err != nil {
		return nil, err
	}
	defer root.Close()
	info, err := root.Stat(relDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("local: stat dir %q: %w", prefix, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("local: %q is not a directory", prefix)
	}
	// os.Root has no ReadDir; open the directory through the root (which
	// refuses traversal out of root) and read its entries from the *os.File.
	df, err := root.Open(relDir)
	if err != nil {
		return nil, fmt.Errorf("local: open dir %q: %w", prefix, err)
	}
	entries, err := df.ReadDir(-1)
	_ = df.Close()
	if err != nil {
		return nil, fmt.Errorf("local: read dir %q: %w", prefix, err)
	}
	base := relDir
	if base == "." {
		base = ""
	}
	out := make([]driver.Entry, 0, len(entries))
	for _, e := range entries {
		key := filepath.ToSlash(filepath.Join(base, e.Name()))
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
