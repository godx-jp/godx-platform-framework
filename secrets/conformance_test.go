package secrets

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"testing"

	sdriver "github.com/godx-jp/godx-platform-framework/secrets/driver"

	envdrv "github.com/godx-jp/godx-platform-framework/secrets/drivers/env"
	filedrv "github.com/godx-jp/godx-platform-framework/secrets/drivers/file"
)

// driverCase is one entry in the conformance matrix.
type driverCase struct {
	name string
	// build returns a fresh Store. The store must be ready for use.
	build func(t *testing.T) sdriver.Store
	// writable indicates whether Put/Forget are supported.
	writable bool
	// listable indicates whether List enumerates all keys.
	listable bool
	// seed allows the test to pre-populate the backend with values
	// (used for read-only stores).
	seed func(t *testing.T, key string, value []byte)
}

func envCase() driverCase {
	const prefix = "SECRETS_CONFORM_"
	return driverCase{
		name:     sdriver.DriverEnv,
		writable: false,
		listable: false,
		build: func(t *testing.T) sdriver.Store {
			return envdrv.New(prefix)
		},
		seed: func(t *testing.T, key string, value []byte) {
			// env driver upper-cases / underscore-replaces keys.
			t.Setenv(prefix+upperEnvKey(key), string(value))
		},
	}
}

func upperEnvKey(k string) string {
	out := []byte{}
	for i := 0; i < len(k); i++ {
		c := k[i]
		switch c {
		case '/', '.', '-', ' ':
			out = append(out, '_')
		default:
			if c >= 'a' && c <= 'z' {
				c = c - 'a' + 'A'
			}
			out = append(out, c)
		}
	}
	return string(out)
}

func fileCase() driverCase {
	return driverCase{
		name:     sdriver.DriverFile,
		writable: true,
		listable: true,
		build: func(t *testing.T) sdriver.Store {
			s, err := filedrv.New(t.TempDir())
			if err != nil {
				t.Fatalf("file driver new: %v", err)
			}
			return s
		},
		seed: func(t *testing.T, key string, value []byte) {
			t.Fatalf("file driver is writable; use Put rather than seed")
		},
	}
}

func runConformance(t *testing.T, dc driverCase) {
	t.Run(dc.name, func(t *testing.T) {
		t.Run("get_missing_returns_not_found", func(t *testing.T) {
			s := dc.build(t)
			defer s.Shutdown(context.Background())
			_, err := s.Get(context.Background(), "missing-key-deadbeef")
			if !errors.Is(err, sdriver.ErrNotFound) {
				t.Fatalf("err=%v", err)
			}
		})

		t.Run("name_matches_driver", func(t *testing.T) {
			s := dc.build(t)
			defer s.Shutdown(context.Background())
			if s.Name() != dc.name {
				t.Fatalf("Name=%q want %q", s.Name(), dc.name)
			}
		})

		t.Run("get_returns_seeded_value", func(t *testing.T) {
			s := dc.build(t)
			defer s.Shutdown(context.Background())
			if dc.writable {
				if err := s.Put(context.Background(), "alpha", []byte("v1")); err != nil {
					t.Fatalf("Put: %v", err)
				}
			} else {
				dc.seed(t, "alpha", []byte("v1"))
			}
			v, err := s.Get(context.Background(), "alpha")
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			if string(v) != "v1" {
				t.Fatalf("v=%q", v)
			}
		})

		t.Run("shutdown_is_idempotent_and_blocks_subsequent_ops", func(t *testing.T) {
			s := dc.build(t)
			if err := s.Shutdown(context.Background()); err != nil {
				t.Fatalf("first: %v", err)
			}
			if err := s.Shutdown(context.Background()); err != nil {
				t.Fatalf("second: %v", err)
			}
			if _, err := s.Get(context.Background(), "k"); !errors.Is(err, sdriver.ErrClosed) {
				t.Fatalf("Get after Shutdown err=%v", err)
			}
		})

		if dc.writable {
			t.Run("put_overwrites", func(t *testing.T) {
				s := dc.build(t)
				defer s.Shutdown(context.Background())
				ctx := context.Background()
				_ = s.Put(ctx, "k", []byte("v1"))
				_ = s.Put(ctx, "k", []byte("v2"))
				v, _ := s.Get(ctx, "k")
				if string(v) != "v2" {
					t.Fatalf("v=%q", v)
				}
			})

			t.Run("forget_missing_is_noop", func(t *testing.T) {
				s := dc.build(t)
				defer s.Shutdown(context.Background())
				if err := s.Forget(context.Background(), "neverexisted"); err != nil {
					t.Fatalf("Forget missing: %v", err)
				}
			})

			t.Run("forget_removes_value", func(t *testing.T) {
				s := dc.build(t)
				defer s.Shutdown(context.Background())
				ctx := context.Background()
				_ = s.Put(ctx, "k", []byte("v"))
				if err := s.Forget(ctx, "k"); err != nil {
					t.Fatalf("Forget: %v", err)
				}
				_, err := s.Get(ctx, "k")
				if !errors.Is(err, sdriver.ErrNotFound) {
					t.Fatalf("err=%v", err)
				}
			})
		} else {
			t.Run("put_rejected_as_read_only", func(t *testing.T) {
				s := dc.build(t)
				defer s.Shutdown(context.Background())
				err := s.Put(context.Background(), "k", []byte("v"))
				if !errors.Is(err, sdriver.ErrReadOnly) {
					t.Fatalf("Put err=%v", err)
				}
				err = s.Forget(context.Background(), "k")
				if !errors.Is(err, sdriver.ErrReadOnly) {
					t.Fatalf("Forget err=%v", err)
				}
			})
		}

		if dc.listable {
			t.Run("list_enumerates_keys", func(t *testing.T) {
				s := dc.build(t)
				defer s.Shutdown(context.Background())
				ctx := context.Background()
				_ = s.Put(ctx, "alpha", []byte("1"))
				_ = s.Put(ctx, "beta", []byte("2"))
				keys, err := s.List(ctx)
				if err != nil {
					t.Fatalf("List: %v", err)
				}
				sort.Strings(keys)
				if len(keys) != 2 || keys[0] != "alpha" || keys[1] != "beta" {
					t.Fatalf("keys=%v", keys)
				}
			})
		} else {
			t.Run("list_returns_not_supported", func(t *testing.T) {
				s := dc.build(t)
				defer s.Shutdown(context.Background())
				keys, err := s.List(context.Background())
				if !errors.Is(err, sdriver.ErrListNotSupported) {
					t.Fatalf("err=%v", err)
				}
				if keys != nil {
					t.Fatalf("keys not nil: %v", keys)
				}
			})
		}
	})
}

func TestConformance(t *testing.T) {
	for _, dc := range []driverCase{envCase(), fileCase()} {
		runConformance(t, dc)
	}
}

// Sanity: light drivers self-register on import.
func TestLightDriversAutoRegister(t *testing.T) {
	for _, n := range []string{sdriver.DriverEnv, sdriver.DriverFile} {
		if sdriver.Lookup(n) == nil {
			t.Fatalf("driver %q not auto-registered", n)
		}
	}
}

// Make sure the file driver normalises behaviour around symlinks /
// trailing newlines etc. by sanity-testing on a real disk write.
func TestFileDriverHandlesTrailingNewline(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "k"), []byte("hello\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	s, _ := filedrv.New(dir)
	defer s.Shutdown(context.Background())
	v, _ := s.Get(context.Background(), "k")
	if string(v) != "hello" {
		t.Fatalf("v=%q", v)
	}
}
