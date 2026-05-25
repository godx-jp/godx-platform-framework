package lock

import (
	"context"
	"fmt"
	"sync"
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
	lockKey := c.prefix + key
	added, err := c.store.Add(ctx, lockKey, c.owner, c.ttl)
	if err != nil {
		return nil, false, err
	}
	if !added {
		return nil, false, nil
	}

	var stopRenew sync.WaitGroup
	stop := make(chan struct{})
	if renewer, ok := c.store.(RenewableStore); ok && c.ttl > 0 {
		interval := c.ttl / 3
		if interval < time.Second {
			interval = time.Second
		}
		stopRenew.Add(1)
		go func() {
			defer stopRenew.Done()
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for {
				select {
				case <-stop:
					return
				case <-ctx.Done():
					return
				case <-ticker.C:
					_ = renewer.Renew(context.Background(), lockKey, c.owner, c.ttl)
				}
			}
		}()
	}

	return func() error {
		close(stop)
		stopRenew.Wait()
		return c.store.Forget(ctx, lockKey)
	}, true, nil
}
