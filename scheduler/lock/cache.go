package lock

import (
	"context"
	"fmt"
	"time"
)

// Cache implements distributed locking via cache Add (set-if-absent).
type Cache struct {
	store  CacheStore
	prefix string
	owner  []byte
	ttl    time.Duration
}

// CacheOptions configures a distributed lock adapter.
type CacheOptions struct {
	Store  CacheStore
	Prefix string
	Owner  string
	TTL    time.Duration
}

// NewCache returns a Cache lock backed by store. owner identifies this
// replica in the stored value; ttl defaults to 24h when zero.
func NewCache(opts CacheOptions) (*Cache, error) {
	if opts.Store == nil {
		return nil, fmt.Errorf("scheduler/lock: CacheStore is required")
	}
	ttl := opts.TTL
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	prefix := opts.Prefix
	if prefix == "" {
		prefix = "schedule-lock:"
	}
	owner := []byte(opts.Owner)
	if len(owner) == 0 {
		owner = []byte("default")
	}
	return &Cache{
		store:  opts.Store,
		prefix: prefix,
		owner:  owner,
		ttl:    ttl,
	}, nil
}

// TryAcquire attempts Add on prefix+key. ok is false when another holder exists.
func (c *Cache) TryAcquire(ctx context.Context, key string) (func() error, bool, error) {
	added, err := c.store.Add(ctx, c.prefix+key, c.owner, c.ttl)
	if err != nil {
		return nil, false, err
	}
	if !added {
		return nil, false, nil
	}
	releaseKey := c.prefix + key
	return func() error {
		return c.store.Forget(ctx, releaseKey)
	}, true, nil
}
