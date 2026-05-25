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

// CacheStore is the subset of cache.Store needed for OnOneServer locks.
// The cache module Store implements this interface.
type CacheStore interface {
	Add(ctx context.Context, key string, value []byte, ttl time.Duration) (added bool, err error)
	Forget(ctx context.Context, key string) error
}
