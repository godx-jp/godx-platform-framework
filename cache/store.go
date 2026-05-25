package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	cdriver "github.com/godx-jp/godx-platform-framework/cache/driver"
)

// Store is the user-facing handle for one named cache backend. Mirrors
// the surface of Laravel's Illuminate\Contracts\Cache\Repository so a
// PHP team moving to Go finds the API immediately familiar:
//
//	Get / Put / Add / Pull / Forever / Forget / Has / Missing / Flush
//	Remember / RememberForever / Increment / Decrement
//
// JSON convenience helpers (GetJSON, PutJSON, RememberJSON) handle
// the common case of storing marshalled structs without forcing
// every caller to repeat the encoding boilerplate.
type Store struct {
	name string
	cfg  StoreConfig
	drv  cdriver.Driver
}

// NewStore wraps an already-constructed driver. Use this when wiring a
// driver outside the Manager (tests, custom flows). For the normal
// boot path, prefer cache.Module which builds Stores from env config
// and registers them on a Manager.
func NewStore(name string, drv cdriver.Driver, cfg StoreConfig) *Store {
	return &Store{name: name, cfg: cfg, drv: drv}
}

// Name returns the store's logical name.
func (s *Store) Name() string { return s.name }

// Driver returns the underlying driver. Useful for tests and for
// advanced callers that need backend-specific extensions; prefer the
// public Store methods otherwise.
func (s *Store) Driver() cdriver.Driver { return s.drv }

// Config returns the configuration the Store was constructed with.
func (s *Store) Config() StoreConfig { return s.cfg }

// ──────────────────────────────────────────────────────────────────
//                      core Laravel-style API
// ──────────────────────────────────────────────────────────────────

// Get returns the value for key. ok == false when missing or expired.
func (s *Store) Get(ctx context.Context, key string) ([]byte, bool, error) {
	return s.drv.Get(ctx, key)
}

// Put stores value. A ttl of 0 stores forever. A negative ttl is
// treated as 0.
func (s *Store) Put(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return s.drv.Put(ctx, key, value, clampTTL(ttl))
}

// Forever is the explicit form of Put with ttl == 0.
func (s *Store) Forever(ctx context.Context, key string, value []byte) error {
	return s.drv.Put(ctx, key, value, 0)
}

// Add stores value only when key is absent (or expired). Returns true
// if the value was written.
func (s *Store) Add(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error) {
	return s.drv.Add(ctx, key, value, clampTTL(ttl))
}

// Forget removes key. Missing keys are not errors.
func (s *Store) Forget(ctx context.Context, key string) error {
	return s.drv.Forget(ctx, key)
}

// Has reports whether key exists and is unexpired.
func (s *Store) Has(ctx context.Context, key string) (bool, error) {
	return s.drv.Has(ctx, key)
}

// Missing is the boolean inverse of Has — useful for predicates that
// read better in the negative.
func (s *Store) Missing(ctx context.Context, key string) (bool, error) {
	ok, err := s.drv.Has(ctx, key)
	return !ok, err
}

// Pull reads and immediately deletes key. Matches Laravel
// Cache::pull — a common pattern for one-shot flash data.
func (s *Store) Pull(ctx context.Context, key string) ([]byte, bool, error) {
	v, ok, err := s.drv.Get(ctx, key)
	if err != nil || !ok {
		return v, ok, err
	}
	if ferr := s.drv.Forget(ctx, key); ferr != nil {
		return v, true, ferr
	}
	return v, true, nil
}

// Flush removes every key owned by this store.
func (s *Store) Flush(ctx context.Context) error {
	return s.drv.Flush(ctx)
}

// Increment / Decrement adjust an integer counter atomically.
// Underlying value must be a decimal integer (or absent).
func (s *Store) Increment(ctx context.Context, key string, delta int64) (int64, error) {
	return s.drv.Increment(ctx, key, delta)
}
func (s *Store) Decrement(ctx context.Context, key string, delta int64) (int64, error) {
	return s.drv.Decrement(ctx, key, delta)
}

// Remember returns the cached value for key, otherwise calls fn to
// produce a fresh value, stores it under key with ttl, and returns it.
// Matches Laravel Cache::remember.
func (s *Store) Remember(ctx context.Context, key string, ttl time.Duration, fn func(context.Context) ([]byte, error)) ([]byte, error) {
	v, ok, err := s.drv.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if ok {
		return v, nil
	}
	fresh, err := fn(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.drv.Put(ctx, key, fresh, clampTTL(ttl)); err != nil {
		return nil, err
	}
	return fresh, nil
}

// RememberForever is Remember with ttl == 0.
func (s *Store) RememberForever(ctx context.Context, key string, fn func(context.Context) ([]byte, error)) ([]byte, error) {
	return s.Remember(ctx, key, 0, fn)
}

// Shutdown releases backend resources. Manager.Shutdown calls this on
// every registered store at app shutdown time.
func (s *Store) Shutdown(ctx context.Context) error {
	return s.drv.Shutdown(ctx)
}

// ──────────────────────────────────────────────────────────────────
//                    JSON convenience helpers
// ──────────────────────────────────────────────────────────────────

// GetJSON unmarshals the cached value into dst.
// ok == false when key is missing/expired; err covers backend and
// unmarshal failures.
func (s *Store) GetJSON(ctx context.Context, key string, dst any) (bool, error) {
	v, ok, err := s.drv.Get(ctx, key)
	if err != nil || !ok {
		return ok, err
	}
	if err := json.Unmarshal(v, dst); err != nil {
		return true, fmt.Errorf("cache: GetJSON unmarshal %q: %w", key, err)
	}
	return true, nil
}

// PutJSON marshals value as JSON and stores it. ttl == 0 stores
// forever.
func (s *Store) PutJSON(ctx context.Context, key string, value any, ttl time.Duration) error {
	b, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("cache: PutJSON marshal %q: %w", key, err)
	}
	return s.drv.Put(ctx, key, b, clampTTL(ttl))
}

// RememberJSON returns the cached value (unmarshalled into dst) when
// present, otherwise calls fn for the fresh value, marshals it,
// stores it, and unmarshals into dst.
func (s *Store) RememberJSON(ctx context.Context, key string, ttl time.Duration, dst any, fn func(context.Context) (any, error)) error {
	if ok, err := s.GetJSON(ctx, key, dst); err != nil || ok {
		return err
	}
	fresh, err := fn(ctx)
	if err != nil {
		return err
	}
	b, err := json.Marshal(fresh)
	if err != nil {
		return fmt.Errorf("cache: RememberJSON marshal %q: %w", key, err)
	}
	if err := s.drv.Put(ctx, key, b, clampTTL(ttl)); err != nil {
		return err
	}
	return json.Unmarshal(b, dst)
}

func clampTTL(ttl time.Duration) time.Duration {
	if ttl < 0 {
		return 0
	}
	return ttl
}
