// Package redis provides distributed scheduler locks via Redis SET NX EX.
//
// Blank-import is not required; construct with lock.NewRedis.
package lock

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

var renewScript = goredis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("SET", KEYS[1], ARGV[1], "EX", ARGV[2])
end
return 0
`)

// Redis implements Mutex using Redis atomic set-if-absent.
type Redis struct {
	client *goredis.Client
	prefix string
	owner  string
	ttl    time.Duration
}

// RedisOptions configures a Redis lock.
type RedisOptions struct {
	Client *goredis.Client
	Prefix string
	Owner  string
	TTL    time.Duration
}

// NewRedis returns a Redis-backed distributed lock.
func NewRedis(opts RedisOptions) (*Redis, error) {
	if opts.Client == nil {
		return nil, fmt.Errorf("scheduler/lock: redis client is required")
	}
	ttl := opts.TTL
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	prefix := opts.Prefix
	if prefix == "" {
		prefix = "schedule-lock:"
	}
	owner := opts.Owner
	if owner == "" {
		owner = "default"
	}
	return &Redis{client: opts.Client, prefix: prefix, owner: owner, ttl: ttl}, nil
}

func (r *Redis) lockKey(key string) string { return r.prefix + key }

// TryAcquire sets key when absent. ok is false when another holder exists.
func (r *Redis) TryAcquire(ctx context.Context, key string) (func() error, bool, error) {
	lockKey := r.lockKey(key)
	ok, err := r.client.SetNX(ctx, lockKey, r.owner, r.ttl).Result()
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, nil
	}
	stop := make(chan struct{})
	go r.renewLoop(stop, lockKey)
	return func() error {
		close(stop)
		return r.client.Del(context.Background(), lockKey).Err()
	}, true, nil
}

func (r *Redis) renewLoop(stop <-chan struct{}, lockKey string) {
	interval := r.ttl / 3
	if interval < time.Second {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			_, _ = renewScript.Run(context.Background(), r.client, []string{lockKey},
				r.owner, int64(r.ttl/time.Second)).Result()
		}
	}
}

// RedisStore adapts Redis for CacheStore/RenewableStore (Add/Renew/Forget).
type RedisStore struct {
	client *goredis.Client
}

// NewRedisStore wraps a Redis client for scheduler.Module cache lock wiring.
func NewRedisStore(client *goredis.Client) *RedisStore {
	return &RedisStore{client: client}
}

func (s *RedisStore) Add(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error) {
	ok, err := s.client.SetNX(ctx, key, value, ttl).Result()
	return ok, err
}

func (s *RedisStore) Forget(ctx context.Context, key string) error {
	return s.client.Del(ctx, key).Err()
}

func (s *RedisStore) Renew(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	res, err := renewScript.Run(ctx, s.client, []string{key}, string(value), int64(ttl/time.Second)).Result()
	if err != nil {
		return err
	}
	if res == int64(0) {
		return fmt.Errorf("scheduler/lock: renew lost key %q", key)
	}
	return nil
}
