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
package file

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
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

func (s *store) path(key string) (string, error) {
	if key == "" {
		return "", sdriver.ErrNotFound
	}
	clean := filepath.ToSlash(filepath.Clean(key))
	if strings.HasPrefix(clean, "../") || clean == ".." || strings.Contains(clean, "/../") || strings.HasPrefix(clean, "/") {
		return "", fmt.Errorf("secrets/file: invalid key %q (must not escape root)", key)
	}
	full := filepath.Join(s.root, filepath.FromSlash(clean))
	rel, err := filepath.Rel(s.root, full)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("secrets/file: invalid key %q (must not escape root)", key)
	}
	return full, nil
}

func (s *store) Get(ctx context.Context, key string) ([]byte, error) {
	if err := s.checkOpen(); err != nil {
		return nil, err
	}
	p, err := s.path(key)
	if err != nil {
		if errors.Is(err, sdriver.ErrNotFound) {
			return nil, err
		}
		return nil, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, sdriver.ErrNotFound
		}
		return nil, fmt.Errorf("secrets/file: read %s: %w", p, err)
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
	p, err := s.path(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return fmt.Errorf("secrets/file: mkdir: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), ".tmp-*")
	if err != nil {
		return fmt.Errorf("secrets/file: tmp: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }
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
	if err := os.Rename(tmpName, p); err != nil {
		cleanup()
		return fmt.Errorf("secrets/file: rename: %w", err)
	}
	return nil
}

func (s *store) Forget(ctx context.Context, key string) error {
	if err := s.checkOpen(); err != nil {
		return err
	}
	p, err := s.path(key)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil {
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
	var out []string
	err := filepath.WalkDir(s.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".tmp-") {
			return nil
		}
		rel, err := filepath.Rel(s.root, path)
		if err != nil {
			return err
		}
		out = append(out, filepath.ToSlash(rel))
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
