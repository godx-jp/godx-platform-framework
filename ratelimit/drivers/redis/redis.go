// Package redis is a distributed token-bucket limiter backed by Redis.
// Token refill and consumption run atomically in a Lua script so
// multiple replicas share one limit.
//
// Blank-import this package to register the driver:
//
//	import _ "github.com/godx-jp/godx-platform-framework/ratelimit/drivers/redis"
package redis

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	goredis "github.com/redis/go-redis/v9"

	rdriver "github.com/godx-jp/godx-platform-framework/ratelimit/driver"
)

const (
	defaultRate  = 10.0
	defaultBurst = 20
)

// tokenBucketScript atomically refills and consumes one token.
var tokenBucketScript = goredis.NewScript(`
local key = KEYS[1]
local rate = tonumber(ARGV[1])
local capacity = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local ttl = tonumber(ARGV[4])

local data = redis.call('HMGET', key, 'tokens', 'last')
local tokens = tonumber(data[1])
local last = tonumber(data[2])

if tokens == nil then
  tokens = capacity
  last = now
else
  local delta = math.max(0, now - last)
  tokens = math.min(capacity, tokens + delta * rate)
  last = now
end

local allowed = 0
if tokens >= 1 then
  tokens = tokens - 1
  allowed = 1
end

redis.call('HSET', key, 'tokens', tokens, 'last', last)
redis.call('EXPIRE', key, ttl)
return allowed
`)

func init() {
	rdriver.Register(rdriver.DriverRedis, NewFromSpec)
}

type limiter struct {
	client *goredis.Client
	prefix string
	rate   float64
	burst  float64
	ttlSec int64

	mu     sync.Mutex
	closed bool
}

// NewFromSpec builds a redis limiter and pings the server.
func NewFromSpec(ctx context.Context, spec rdriver.Spec) (rdriver.Limiter, error) {
	opts, err := buildOptions(spec)
	if err != nil {
		return nil, err
	}
	client := goredis.NewClient(opts)
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ratelimit/redis: ping: %w", err)
	}
	rate := spec.Rate
	if rate <= 0 {
		rate = defaultRate
	}
	burst := spec.Burst
	if burst <= 0 {
		burst = defaultBurst
	}
	ttl := spec.TTL
	if ttl <= 0 {
		ttl = time.Duration(float64(burst)/rate)*time.Second + time.Second
		if ttl < 2*time.Second {
			ttl = 2 * time.Second
		}
	}
	prefix := spec.Prefix
	if prefix == "" {
		prefix = "ratelimit:"
	}
	return &limiter{
		client: client,
		prefix: prefix,
		rate:   rate,
		burst:  float64(burst),
		ttlSec: int64(ttl.Seconds()),
	}, nil
}

func (l *limiter) Name() string { return rdriver.DriverRedis }

func (l *limiter) Allow(ctx context.Context, key string) (bool, error) {
	if err := l.checkOpen(); err != nil {
		return false, err
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if key == "" {
		key = "_"
	}
	now := float64(time.Now().UnixNano()) / 1e9
	res, err := tokenBucketScript.Run(ctx, l.client, []string{l.redisKey(key)},
		l.rate, l.burst, now, l.ttlSec,
	).Int()
	if err != nil {
		return false, err
	}
	return res == 1, nil
}

func (l *limiter) Reset(ctx context.Context, key string) {
	if err := l.checkOpen(); err != nil {
		return
	}
	if key == "" {
		key = "_"
	}
	_ = l.client.Del(ctx, l.redisKey(key)).Err()
}

func (l *limiter) Shutdown(_ context.Context) error {
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return nil
	}
	l.closed = true
	client := l.client
	l.mu.Unlock()
	return client.Close()
}

func (l *limiter) redisKey(key string) string {
	return l.prefix + key
}

func (l *limiter) checkOpen() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return rdriver.ErrClosed
	}
	return nil
}

func buildOptions(spec rdriver.Spec) (*goredis.Options, error) {
	if strings.TrimSpace(spec.URL) != "" {
		opts, err := goredis.ParseURL(spec.URL)
		if err != nil {
			return nil, fmt.Errorf("ratelimit/redis: parse URL: %w", err)
		}
		return opts, nil
	}
	addr := strings.TrimSpace(spec.Address)
	if addr == "" {
		return nil, fmt.Errorf("ratelimit/redis: URL or ADDRESS is required")
	}
	return &goredis.Options{
		Addr:     addr,
		Username: spec.Username,
		Password: spec.Password,
		DB:       spec.DB,
	}, nil
}

// RedisKey returns the fully prefixed Redis key (for tests).
func RedisKey(prefix, key string) string {
	if prefix == "" {
		prefix = "ratelimit:"
	}
	if key == "" {
		key = "_"
	}
	return prefix + key
}

// ParseDB is exported for tests that build URLs manually.
func ParseDB(s string) (int, error) {
	if s == "" {
		return 0, nil
	}
	return strconv.Atoi(strings.TrimPrefix(s, "/"))
}
