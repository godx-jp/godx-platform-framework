// Package file is the on-disk secrets store, designed to interoperate
// with Kubernetes secret volume mounts and Docker secret files. Each
// secret is one file under spec.Path; the file's full content (with
// the trailing newline trimmed) becomes the secret value.
//
// Sub-keys are encoded as directories — Get("db/password") reads
// <root>/db/password. The driver refuses to traverse outside its
// root (no "../" segments).
//
// Put writes 0600-mode files atomically (write-temp-then-rename) and
// Forget removes them. Pass spec.Prefix to enforce a fixed root
// subdirectory.
//
// All filesystem access is confined to the root with os.Root
// (Go 1.24+): os.Root refuses to traverse symlinks that resolve
// outside the root, so a symlink planted inside a shared volume cannot
// redirect a Get to an out-of-root target such as /etc/shadow. The
// lexical key validation in path is kept as defense-in-depth.
package file

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	sdriver "github.com/godx-jp/godx-platform-framework/secrets/driver"
)

func init() {
	sdriver.Register(sdriver.DriverFile, func(_ context.Context, spec sdriver.Spec) (sdriver.Store, error) {
		if spec.Path == "" {
			return nil, fmt.Errorf("secrets/file: spec.Path is required")
		}
		root := spec.Path
		if spec.Prefix != "" {
			root = filepath.Join(root, spec.Prefix)
		}
		return New(root)
	})
}

type store struct {
	root string

	mu     sync.Mutex
	closed bool
}

// New constructs a file Store rooted at root. The directory is
// created (with 0700 permission) if it does not exist.
func New(root string) (sdriver.Store, error) {
	if root == "" {
		return nil, fmt.Errorf("secrets/file: root is required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("secrets/file: %w", err)
	}
	if err := os.MkdirAll(abs, 0o700); err != nil {
		return nil, fmt.Errorf("secrets/file: mkdir: %w", err)
	}
	return &store{root: abs}, nil
}

func (s *store) Name() string { return sdriver.DriverFile }

// relKey validates key lexically and returns a slash-separated path
// relative to the root, suitable for the *os.Root methods. This is
// defense-in-depth: os.Root itself refuses to escape the root, but we
// reject obviously bad keys early for clearer errors.
func (s *store) relKey(key string) (string, error) {
	if key == "" {
		return "", sdriver.ErrNotFound
	}
	clean := filepath.ToSlash(filepath.Clean(key))
	if strings.HasPrefix(clean, "../") || clean == ".." || strings.Contains(clean, "/../") || strings.HasPrefix(clean, "/") {
		return "", fmt.Errorf("secrets/file: invalid key %q (must not escape root)", key)
	}
	return clean, nil
}

func (s *store) Get(ctx context.Context, key string) ([]byte, error) {
	if err := s.checkOpen(); err != nil {
		return nil, err
	}
	rel, err := s.relKey(key)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(s.root)
	if err != nil {
		return nil, fmt.Errorf("secrets/file: open root: %w", err)
	}
	defer root.Close()
	// root.Open refuses to traverse a symlink that escapes the root.
	f, err := root.Open(rel)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, sdriver.ErrNotFound
		}
		return nil, fmt.Errorf("secrets/file: read %s: %w", rel, err)
	}
	defer f.Close()
	b, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("secrets/file: read %s: %w", rel, err)
	}
	if n := len(b); n > 0 && b[n-1] == '\n' {
		b = b[:n-1]
	}
	return b, nil
}

func (s *store) Put(ctx context.Context, key string, value []byte) error {
	if err := s.checkOpen(); err != nil {
		return err
	}
	rel, err := s.relKey(key)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(s.root)
	if err != nil {
		return fmt.Errorf("secrets/file: open root: %w", err)
	}
	defer root.Close()

	dir := path.Dir(rel)
	if dir != "." {
		if err := root.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("secrets/file: mkdir: %w", err)
		}
	}

	// Atomic write: create a unique temp file via the confined root,
	// write+chmod, then rename over the target — all within the root so
	// os.Root prevents any symlink from redirecting the write.
	suffix, err := randSuffix()
	if err != nil {
		return fmt.Errorf("secrets/file: tmp: %w", err)
	}
	tmpName := path.Join(dir, ".tmp-"+suffix)
	tmp, err := root.OpenFile(tmpName, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("secrets/file: tmp: %w", err)
	}
	cleanup := func() { _ = root.Remove(tmpName) }
	if _, err := tmp.Write(value); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf("secrets/file: write: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf("secrets/file: chmod: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("secrets/file: close: %w", err)
	}
	if err := root.Rename(tmpName, rel); err != nil {
		cleanup()
		return fmt.Errorf("secrets/file: rename: %w", err)
	}
	return nil
}

// randSuffix returns a short random hex string for temp file names.
func randSuffix() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func (s *store) Forget(ctx context.Context, key string) error {
	if err := s.checkOpen(); err != nil {
		return err
	}
	rel, err := s.relKey(key)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(s.root)
	if err != nil {
		return fmt.Errorf("secrets/file: open root: %w", err)
	}
	defer root.Close()
	if err := root.Remove(rel); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("secrets/file: remove: %w", err)
	}
	return nil
}

func (s *store) List(ctx context.Context) ([]string, error) {
	if err := s.checkOpen(); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(s.root)
	if err != nil {
		return nil, fmt.Errorf("secrets/file: open root: %w", err)
	}
	defer root.Close()
	var out []string
	// fs.WalkDir over the confined root.FS() never descends through
	// symlinks, so an out-of-root symlink is reported (and skipped as a
	// non-regular entry) but never traversed.
	err = fs.WalkDir(root.FS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		// Only regular files are secrets; skip symlinks and other
		// non-regular entries planted in the root.
		if !d.Type().IsRegular() {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".tmp-") {
			return nil
		}
		out = append(out, p)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("secrets/file: walk: %w", err)
	}
	return out, nil
}

func (s *store) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	return nil
}

func (s *store) checkOpen() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return sdriver.ErrClosed
	}
	return nil
}
