package lock

import (
	"context"
	"fmt"
	"log/slog"
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
//
// Each call stores a unique per-acquisition token (owner + random suffix), not
// the bare configured owner, so two replicas that share the same Owner never
// produce the same lock value. Renew is value-checked against this exact
// token, so a holder that has lost the lock cannot re-extend one another
// replica now owns.
func (c *Cache) TryAcquire(ctx context.Context, key string) (func() error, bool, error) {
	// Delegate to TryAcquireLease so there is a single acquisition + renewal
	// implementation; callers of TryAcquire simply discard the lost channel.
	release, _, ok, err := c.TryAcquireLease(ctx, key)
	return release, ok, err
}

// TryAcquireLease behaves like TryAcquire but also returns a channel that is
// closed exactly once when the renew loop can no longer guarantee ownership
// (a value-checked Renew error: the TTL expired or another replica took the
// lock over). The holder can select on this channel to abort in-flight work
// and uphold the OnOneServer guarantee.
//
// When the store does not implement RenewableStore (or ttl <= 0) there is no
// renewer, so the lost channel is never closed for the life of the lock; the
// release closure stops the (absent) renewer and forgets the key as before.
func (c *Cache) TryAcquireLease(ctx context.Context, key string) (func() error, <-chan struct{}, bool, error) {
	lockKey := c.prefix + key
	suffix, err := randomToken()
	if err != nil {
		return nil, nil, false, fmt.Errorf("scheduler/lock: generate lock token: %w", err)
	}
	token := append(append([]byte(nil), c.owner...), ':')
	token = append(token, suffix...)
	added, err := c.store.Add(ctx, lockKey, token, c.ttl)
	if err != nil {
		return nil, nil, false, err
	}
	if !added {
		return nil, nil, false, nil
	}

	lost := make(chan struct{})
	var lostOnce sync.Once
	signalLost := func() { lostOnce.Do(func() { close(lost) }) }

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
					// SECURITY: a renew error means the lock was lost (TTL
					// expired or another replica took it over via a
					// value-checked Renew) while the job is still running, so
					// the OnOneServer guarantee is temporarily broken. We log
					// it for operators AND signal the lost channel so the
					// holder (scheduler.runJob) can cancel the in-flight job.
					if err := renewer.Renew(context.Background(), lockKey, token, c.ttl); err != nil {
						slog.Warn("scheduler/lock: cache lock renew failed; lock may expire mid-job, OnOneServer no longer guaranteed",
							"key", lockKey, "error", err)
						signalLost()
					}
				}
			}
		}()
	}

	return func() error {
		close(stop)
		stopRenew.Wait()
		return c.store.Forget(ctx, lockKey)
	}, lost, true, nil
}
