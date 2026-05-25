// Package env is the OS-environment secrets store. Keys are
// upper-cased, dashes / dots / slashes are replaced with single
// underscores, and the configured prefix (default SECRETS_) is
// prepended. So Get("db/password") on a default-prefix store reads
// SECRETS_DB_PASSWORD from the environment.
//
// The env driver is intentionally read-only: production secrets
// should never be mutated through environment writes (which would
// not propagate to other processes anyway). Put and Forget always
// return driver.ErrReadOnly.
package env

import (
	"context"
	"os"
	"strings"
	"sync"

	sdriver "github.com/godx-jp/godx-platform-framework/secrets/driver"
)

// DefaultPrefix is applied to every key when spec.Prefix is empty.
const DefaultPrefix = "SECRETS_"

func init() {
	sdriver.Register(sdriver.DriverEnv, func(_ context.Context, spec sdriver.Spec) (sdriver.Store, error) {
		return New(spec.Prefix), nil
	})
}

type store struct {
	prefix string

	mu     sync.Mutex
	closed bool
}

// New constructs an env Store. When prefix is "" the DefaultPrefix
// (SECRETS_) is used. Pass a literal "-" to use no prefix at all.
func New(prefix string) sdriver.Store {
	switch prefix {
	case "":
		prefix = DefaultPrefix
	case "-":
		prefix = ""
	}
	return &store{prefix: prefix}
}

func (s *store) Name() string { return sdriver.DriverEnv }

// Normalize turns a logical key (db/password, db.password,
// auth-token) into the environment variable name actually read.
func (s *store) envKey(key string) string {
	k := strings.ToUpper(key)
	for _, sep := range []string{"/", ".", "-", " "} {
		k = strings.ReplaceAll(k, sep, "_")
	}
	return s.prefix + k
}

func (s *store) Get(ctx context.Context, key string) ([]byte, error) {
	if err := s.checkOpen(); err != nil {
		return nil, err
	}
	if key == "" {
		return nil, sdriver.ErrNotFound
	}
	v, ok := os.LookupEnv(s.envKey(key))
	if !ok {
		return nil, sdriver.ErrNotFound
	}
	return []byte(v), nil
}

func (s *store) Put(ctx context.Context, key string, value []byte) error {
	if err := s.checkOpen(); err != nil {
		return err
	}
	return sdriver.ErrReadOnly
}

func (s *store) Forget(ctx context.Context, key string) error {
	if err := s.checkOpen(); err != nil {
		return err
	}
	return sdriver.ErrReadOnly
}

func (s *store) List(ctx context.Context) ([]string, error) {
	if err := s.checkOpen(); err != nil {
		return nil, err
	}
	return nil, sdriver.ErrListNotSupported
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
