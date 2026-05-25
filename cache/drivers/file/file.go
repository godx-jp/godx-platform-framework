// Package file provides a filesystem-backed cache driver.
//
// Storage layout matches Laravel's FileStore — one file per cache key,
// containing a JSON envelope with the expiry timestamp and the
// base64-encoded payload. Filenames use the SHA-1 of the full
// (prefixed) key, sharded two levels deep to keep directories
// browsable on backends with per-dir entry limits (FAT, some NFS
// mounts):
//
//	<root>/aa/bb/aabb…ee.cache
//
// Light driver — no third-party dependencies; auto-registers under the
// name "file" via the parent cache.register import.
package file

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	cdriver "github.com/godx-jp/godx-platform-framework/cache/driver"
)

func init() {
	cdriver.Register(cdriver.DriverFile, construct)
}

// Name is exported so callers can reference the driver name without
// hard-coding a string.
const Name = cdriver.DriverFile

const fileExt = ".cache"

// envelope is the on-disk representation of a cache entry. The schema
// is stable across versions — adding fields stays backward compatible
// as long as existing ones are not renamed.
type envelope struct {
	// ExpiresAtUnixMilli is 0 when the entry never expires.
	ExpiresAtUnixMilli int64  `json:"exp"`
	ValueBase64        []byte `json:"val"`
}

func construct(_ context.Context, spec cdriver.Spec) (cdriver.Driver, error) {
	root := strings.TrimSpace(spec.Path)
	if root == "" {
		return nil, fmt.Errorf("file: path is required")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("file: mkdir %q: %w", root, err)
	}
	return &impl{root: root, prefix: spec.Prefix}, nil
}

type impl struct {
	root   string
	prefix string

	// keyLock serialises writes for a given key so Add and Increment
	// remain atomic between read and write halves on the same process.
	// Cross-process safety relies on os.Rename's atomicity within a
	// single filesystem, which it is on Linux/macOS/Windows for the
	// rename-into-place pattern used below.
	keyLock keyMutex
}

func (d *impl) fullKey(key string) string { return d.prefix + key }

func (d *impl) path(key string) string {
	hash := sha1.Sum([]byte(d.fullKey(key)))
	hex := hex.EncodeToString(hash[:])
	return filepath.Join(d.root, hex[:2], hex[2:4], hex+fileExt)
}

func (d *impl) Get(_ context.Context, key string) ([]byte, bool, error) {
	env, ok, err := d.read(key)
	if err != nil || !ok {
		return nil, ok, err
	}
	return env.ValueBase64, true, nil
}

func (d *impl) read(key string) (envelope, bool, error) {
	path := d.path(key)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return envelope{}, false, nil
		}
		return envelope{}, false, fmt.Errorf("file: read %q: %w", path, err)
	}
	var env envelope
	if err := json.Unmarshal(data, &env); err != nil {
		// Treat corrupt envelopes as a miss; remove so future writes
		// succeed cleanly. Surface the underlying error for callers
		// that want to log it.
		_ = os.Remove(path)
		return envelope{}, false, fmt.Errorf("file: corrupt envelope %q: %w", path, err)
	}
	if env.ExpiresAtUnixMilli > 0 && time.Now().UnixMilli() >= env.ExpiresAtUnixMilli {
		_ = os.Remove(path)
		return envelope{}, false, nil
	}
	return env, true, nil
}

func (d *impl) Put(_ context.Context, key string, val []byte, ttl time.Duration) error {
	d.keyLock.lock(key)
	defer d.keyLock.unlock(key)
	return d.writeEnvelope(key, envelopeFromValue(val, ttl))
}

func envelopeFromValue(val []byte, ttl time.Duration) envelope {
	env := envelope{ValueBase64: append([]byte(nil), val...)}
	if ttl > 0 {
		env.ExpiresAtUnixMilli = time.Now().Add(ttl).UnixMilli()
	}
	return env
}

func (d *impl) writeEnvelope(key string, env envelope) error {
	path := d.path(key)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("file: mkdir %q: %w", filepath.Dir(path), err)
	}
	data, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("file: marshal envelope: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".cache-*")
	if err != nil {
		return fmt.Errorf("file: create temp: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("file: write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("file: close temp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("file: rename %q -> %q: %w", tmpPath, path, err)
	}
	return nil
}

func (d *impl) Add(_ context.Context, key string, val []byte, ttl time.Duration) (bool, error) {
	d.keyLock.lock(key)
	defer d.keyLock.unlock(key)
	if _, ok, err := d.read(key); err != nil {
		// Corruption — treat as missing so Add can repopulate.
		_ = err
	} else if ok {
		return false, nil
	}
	return true, d.writeEnvelope(key, envelopeFromValue(val, ttl))
}

func (d *impl) Forget(_ context.Context, key string) error {
	d.keyLock.lock(key)
	defer d.keyLock.unlock(key)
	if err := os.Remove(d.path(key)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("file: remove: %w", err)
	}
	return nil
}

func (d *impl) Has(_ context.Context, key string) (bool, error) {
	_, ok, err := d.read(key)
	return ok, err
}

// Flush walks the entire root and removes every *.cache file. Note
// that the file driver cannot distinguish prefixes once keys are
// hashed; the prefix is therefore informational only — Flush wipes the
// whole root. Operators wanting prefix isolation should give each
// logical store its own Path.
func (d *impl) Flush(_ context.Context) error {
	return filepath.WalkDir(d.root, func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if e.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, fileExt) {
			return nil
		}
		if rmErr := os.Remove(path); rmErr != nil && !errors.Is(rmErr, fs.ErrNotExist) {
			return fmt.Errorf("file: flush remove %q: %w", path, rmErr)
		}
		return nil
	})
}

func (d *impl) Increment(_ context.Context, key string, delta int64) (int64, error) {
	return d.adjust(key, delta)
}

func (d *impl) Decrement(_ context.Context, key string, delta int64) (int64, error) {
	return d.adjust(key, -delta)
}

func (d *impl) adjust(key string, delta int64) (int64, error) {
	d.keyLock.lock(key)
	defer d.keyLock.unlock(key)
	env, ok, err := d.read(key)
	if err != nil {
		// Corruption: treat as missing so the counter rebuilds.
		ok = false
	}
	var current int64
	if ok {
		n, perr := strconv.ParseInt(string(env.ValueBase64), 10, 64)
		if perr != nil {
			return 0, fmt.Errorf("%w: key=%q stored=%q", cdriver.ErrNotInteger, key, env.ValueBase64)
		}
		current = n
	}
	current += delta
	newEnv := envelope{
		ValueBase64:        []byte(strconv.FormatInt(current, 10)),
		ExpiresAtUnixMilli: env.ExpiresAtUnixMilli, // preserve TTL across adjustment
	}
	return current, d.writeEnvelope(key, newEnv)
}

func (d *impl) Shutdown(_ context.Context) error {
	return nil
}

// ── keyMutex ────────────────────────────────────────────────────────
//
// keyMutex serialises operations per key without holding one giant
// store-wide lock. It is sized by the number of concurrent keys in
// flight, not the total cardinality, so memory stays bounded.

type keyMutex struct {
	once sync.Once
	mu   sync.Mutex
	locks map[string]*lockEntry
}

type lockEntry struct {
	m       sync.Mutex
	holders int
}

func (k *keyMutex) init() {
	k.once.Do(func() { k.locks = make(map[string]*lockEntry) })
}

func (k *keyMutex) lock(key string) {
	k.init()
	k.mu.Lock()
	e, ok := k.locks[key]
	if !ok {
		e = &lockEntry{}
		k.locks[key] = e
	}
	e.holders++
	k.mu.Unlock()
	e.m.Lock()
}

func (k *keyMutex) unlock(key string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	e, ok := k.locks[key]
	if !ok {
		return
	}
	e.m.Unlock()
	e.holders--
	if e.holders == 0 {
		delete(k.locks, key)
	}
}
