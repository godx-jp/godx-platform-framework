package lock

import (
	"context"
	"time"
)

// Mutex serialises job execution. MemoryLock is process-local;
// CacheLock is distributed via cache.Store Add semantics.
type Mutex interface {
	TryAcquire(ctx context.Context, key string) (release func() error, ok bool, err error)
}

// LeasedMutex is optionally implemented by a Mutex whose held lock can be
// lost while in use (distributed lease expiry / theft). TryAcquireLease
// behaves like TryAcquire but also returns a channel that is closed when the
// renewer can no longer guarantee ownership, so the holder can abort work.
type LeasedMutex interface {
	Mutex
	TryAcquireLease(ctx context.Context, key string) (release func() error, lost <-chan struct{}, ok bool, err error)
}

// CacheStore is the subset of cache.Store needed for OnOneServer locks.
// The cache module Store implements this interface.
type CacheStore interface {
	Add(ctx context.Context, key string, value []byte, ttl time.Duration) (added bool, err error)
	Forget(ctx context.Context, key string) error
}

// RenewableStore extends CacheStore with TTL renewal for long-running jobs.
type RenewableStore interface {
	CacheStore
	Renew(ctx context.Context, key string, value []byte, ttl time.Duration) error
}
