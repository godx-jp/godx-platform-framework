// Package redis provides distributed scheduler locks via Redis SET NX EX.
//
// Blank-import is not required; construct with lock.NewRedis.
package lock

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// renewScript re-extends the TTL only when the stored value still matches
// the caller's token, so a renewal can never resurrect or extend a lock
// already taken over by another replica.
var renewScript = goredis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("SET", KEYS[1], ARGV[1], "EX", ARGV[2])
end
return 0
`)

// releaseScript deletes the key only when the stored value still matches the
// caller's token (compare-and-delete). This prevents a replica from deleting
// a lock that a different replica acquired after this one's lock expired.
var releaseScript = goredis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("DEL", KEYS[1])
end
return 0
`)

// randomToken returns a cryptographically random hex string used to make
// every acquisition's lock value unique, even across replicas that share the
// same configured Owner.
func randomToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

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
//
// Each call generates a unique per-acquisition token (Owner + random) and
// stores it as the lock value. The returned release closure and the renewal
// loop both carry that exact token, so a replica can only ever renew or
// delete the lock it actually holds — never one that another replica took
// over after this lock expired.
func (r *Redis) TryAcquire(ctx context.Context, key string) (func() error, bool, error) {
	lockKey := r.lockKey(key)
	suffix, err := randomToken()
	if err != nil {
		return nil, false, fmt.Errorf("scheduler/lock: generate lock token: %w", err)
	}
	token := r.owner + ":" + suffix
	ok, err := r.client.SetNX(ctx, lockKey, token, r.ttl).Result()
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, nil
	}
	stop := make(chan struct{})
	go r.renewLoop(stop, lockKey, token)
	return func() error {
		close(stop)
		// Compare-and-delete: only remove the key when it still holds our
		// token, so we never delete a lock another replica now owns.
		return releaseScript.Run(context.Background(), r.client, []string{lockKey}, token).Err()
	}, true, nil
}

func (r *Redis) renewLoop(stop <-chan struct{}, lockKey, token string) {
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
			res, err := renewScript.Run(context.Background(), r.client, []string{lockKey},
				token, int64(r.ttl/time.Second)).Result()
			// SECURITY: A renew failure (error) or a 0 result (the key
			// expired or was taken over by another replica) means this
			// holder no longer owns the lock, yet the job is still running —
			// so the OnOneServer guarantee is temporarily lost. We surface it
			// loudly for operators. Cancelling the in-flight job would require
			// threading a cancel signal through scheduler.runJob; until that
			// is wired, the residual risk is logged here.
			switch {
			case err != nil:
				slog.Warn("scheduler/lock: redis lock renew failed; lock may expire mid-job",
					"key", lockKey, "error", err)
			case res == int64(0):
				slog.Warn("scheduler/lock: redis lock lost during job (key expired or stolen); OnOneServer no longer guaranteed",
					"key", lockKey)
			}
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

// Forget removes the lock by key.
//
// SECURITY / LIMITATION: this is an unconditional DEL, not a compare-and-delete,
// because the CacheStore.Forget contract (satisfied by the cache module's
// Store) carries no value/token argument — adding one here would break that
// shared interface and the other modules that implement it. As a result the
// CacheStore path cannot prevent the "release deletes a lock another replica
// now owns" race on its own. Two mitigations keep the residual risk bounded:
// (1) Renew is value-checked (see Renew), so a holder that has already lost
// the lock stops re-extending it; and (2) lock.Cache uses a per-acquisition
// token (see cache.go) so a renewal can never resurrect a stolen lock.
// Callers needing strict compare-and-delete on release should use the direct
// *Redis lock (NewRedis), whose release closure is compare-and-delete.
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
