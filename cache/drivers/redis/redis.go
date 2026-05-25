// Package redis implements a Redis-backed cache driver using
// github.com/redis/go-redis/v9.
//
// HEAVY driver — depends on go-redis. Consumers opt in with a blank
// import:
//
//	import _ "github.com/godx-jp/godx-platform-framework/cache/drivers/redis"
//
// Configuration (see docs/CONFIGURATION.md § Cache):
//
//	CACHE_STORE_<NAME>_DRIVER=redis
//	CACHE_STORE_<NAME>_URL=redis://:secret@127.0.0.1:6379/0   # preferred
//	# or component-wise:
//	# CACHE_STORE_<NAME>_ADDRESS=127.0.0.1:6379
//	# CACHE_STORE_<NAME>_USERNAME=default
//	# CACHE_STORE_<NAME>_PASSWORD=secret
//	# CACHE_STORE_<NAME>_DB=0
//	# CACHE_STORE_<NAME>_TLS=false
//
// Operations
//
//   - Get -> GET; missing key returns ok=false (not an error).
//   - Put -> SET (with PX when TTL > 0).
//   - Add -> SET NX (with PX when TTL > 0).
//   - Forget -> DEL.
//   - Has -> EXISTS.
//   - Flush -> SCAN+UNLINK over the configured prefix (or FLUSHDB
//     when no prefix is set). Scoped to the store's keyspace via
//     the prefix the framework configures.
//   - Increment / Decrement -> INCRBY / DECRBY (native atomic).
package redis

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"

	cdriver "github.com/godx-jp/godx-platform-framework/cache/driver"
)

func init() {
	cdriver.Register(cdriver.DriverRedis, construct)
}

// Name is exported so callers can reference the driver name without
// hard-coding a string.
const Name = cdriver.DriverRedis

func construct(ctx context.Context, spec cdriver.Spec) (cdriver.Driver, error) {
	opts, err := buildOptions(spec)
	if err != nil {
		return nil, err
	}
	client := goredis.NewClient(opts)
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("redis: ping: %w", err)
	}
	return &impl{client: client, prefix: spec.Prefix}, nil
}

func buildOptions(spec cdriver.Spec) (*goredis.Options, error) {
	if strings.TrimSpace(spec.URL) != "" {
		opts, err := goredis.ParseURL(spec.URL)
		if err != nil {
			return nil, fmt.Errorf("redis: parse URL: %w", err)
		}
		return opts, nil
	}
	if strings.TrimSpace(spec.Address) == "" {
		return nil, fmt.Errorf("redis: address (or URL) is required")
	}
	return &goredis.Options{
		Addr:     spec.Address,
		Username: spec.Username,
		Password: spec.Password,
		DB:       spec.DB,
	}, nil
}

type impl struct {
	client *goredis.Client
	prefix string
}

func (d *impl) full(key string) string { return d.prefix + key }

func (d *impl) Get(ctx context.Context, key string) ([]byte, bool, error) {
	val, err := d.client.Get(ctx, d.full(key)).Bytes()
	if err == nil {
		return val, true, nil
	}
	if errors.Is(err, goredis.Nil) {
		return nil, false, nil
	}
	return nil, false, fmt.Errorf("redis: GET %q: %w", key, err)
}

func (d *impl) Put(ctx context.Context, key string, val []byte, ttl time.Duration) error {
	// goredis treats Expiration == 0 as KeepTTL only when KeepTTL is set;
	// the standard SET command with no TTL is exactly what we want when
	// ttl == 0, and the driver naturally maps that to a persist-forever.
	return d.client.Set(ctx, d.full(key), val, ttl).Err()
}

func (d *impl) Add(ctx context.Context, key string, val []byte, ttl time.Duration) (bool, error) {
	ok, err := d.client.SetNX(ctx, d.full(key), val, ttl).Result()
	if err != nil {
		return false, fmt.Errorf("redis: SET NX %q: %w", key, err)
	}
	return ok, nil
}

func (d *impl) Forget(ctx context.Context, key string) error {
	if err := d.client.Del(ctx, d.full(key)).Err(); err != nil {
		return fmt.Errorf("redis: DEL %q: %w", key, err)
	}
	return nil
}

func (d *impl) Has(ctx context.Context, key string) (bool, error) {
	n, err := d.client.Exists(ctx, d.full(key)).Result()
	if err != nil {
		return false, fmt.Errorf("redis: EXISTS %q: %w", key, err)
	}
	return n > 0, nil
}

// Flush deletes every key under the configured prefix. With an empty
// prefix it issues FLUSHDB on the selected logical database — useful
// for tests, dangerous in production. Always set a prefix when sharing
// a Redis instance with other workloads.
func (d *impl) Flush(ctx context.Context) error {
	if d.prefix == "" {
		return d.client.FlushDB(ctx).Err()
	}
	var cursor uint64
	pattern := d.prefix + "*"
	for {
		keys, next, err := d.client.Scan(ctx, cursor, pattern, 256).Result()
		if err != nil {
			return fmt.Errorf("redis: SCAN %q: %w", pattern, err)
		}
		if len(keys) > 0 {
			if err := d.client.Unlink(ctx, keys...).Err(); err != nil {
				return fmt.Errorf("redis: UNLINK: %w", err)
			}
		}
		if next == 0 {
			return nil
		}
		cursor = next
	}
}

func (d *impl) Increment(ctx context.Context, key string, delta int64) (int64, error) {
	v, err := d.client.IncrBy(ctx, d.full(key), delta).Result()
	if err != nil {
		// Redis returns WRONGTYPE when the value isn't an integer.
		if rerr, ok := err.(interface{ Error() string }); ok && strings.HasPrefix(rerr.Error(), "ERR value is not an integer") {
			return 0, fmt.Errorf("%w: key=%q underlying=%v", cdriver.ErrNotInteger, key, err)
		}
		return 0, fmt.Errorf("redis: INCRBY %q: %w", key, err)
	}
	return v, nil
}

func (d *impl) Decrement(ctx context.Context, key string, delta int64) (int64, error) {
	v, err := d.client.DecrBy(ctx, d.full(key), delta).Result()
	if err != nil {
		if rerr, ok := err.(interface{ Error() string }); ok && strings.HasPrefix(rerr.Error(), "ERR value is not an integer") {
			return 0, fmt.Errorf("%w: key=%q underlying=%v", cdriver.ErrNotInteger, key, err)
		}
		return 0, fmt.Errorf("redis: DECRBY %q: %w", key, err)
	}
	return v, nil
}

func (d *impl) Shutdown(_ context.Context) error {
	return d.client.Close()
}
