// Package redis is a distributed queue backed by Redis lists and sorted sets.
//
// Blank-import to register:
//
//	import _ "github.com/godx-jp/godx-platform-framework/queue/drivers/redis"
package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	goredis "github.com/redis/go-redis/v9"

	qdriver "github.com/godx-jp/godx-platform-framework/queue/driver"
)

func init() {
	qdriver.Register(qdriver.DriverRedis, NewFromSpec)
}

type storedJob struct {
	ID       string `json:"id"`
	Queue    string `json:"queue"`
	Payload  []byte `json:"payload"`
	Attempts int    `json:"attempts"`
}

type redisJob struct {
	data storedJob
}

func (j *redisJob) ID() string       { return j.data.ID }
func (j *redisJob) Queue() string    { return j.data.Queue }
func (j *redisJob) Payload() []byte  { return append([]byte(nil), j.data.Payload...) }
func (j *redisJob) Attempts() int    { return j.data.Attempts }

type impl struct {
	client       *goredis.Client
	defaultQueue string
	prefix       string
	nextID       atomic.Uint64

	mu     sync.Mutex
	closed bool
}

func NewFromSpec(ctx context.Context, spec qdriver.Spec) (qdriver.Backend, error) {
	opts, err := buildOptions(spec)
	if err != nil {
		return nil, err
	}
	client := goredis.NewClient(opts)
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("queue/redis: ping: %w", err)
	}
	prefix := spec.Prefix
	if prefix == "" {
		prefix = "godx:queue:"
	}
	dq := spec.DefaultQueue
	if dq == "" {
		dq = "default"
	}
	return &impl{
		client:       client,
		defaultQueue: dq,
		prefix:       prefix,
	}, nil
}

func (d *impl) Name() string { return qdriver.DriverRedis }

func (d *impl) listKey(queue string) string {
	return d.prefix + "list:" + queue
}

func (d *impl) delayedKey(queue string) string {
	return d.prefix + "delayed:" + queue
}

func (d *impl) resolveQueue(queue string) string {
	if queue == "" {
		return d.defaultQueue
	}
	return queue
}

func (d *impl) Push(ctx context.Context, queue string, payload []byte) (qdriver.Job, error) {
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return nil, qdriver.ErrClosed
	}
	d.mu.Unlock()

	queue = d.resolveQueue(queue)
	job := &redisJob{data: storedJob{
		ID:       strconv.FormatUint(d.nextID.Add(1), 10),
		Queue:    queue,
		Payload:  append([]byte(nil), payload...),
		Attempts: 0,
	}}
	raw, err := json.Marshal(job.data)
	if err != nil {
		return nil, err
	}
	if err := d.client.RPush(ctx, d.listKey(queue), raw).Err(); err != nil {
		return nil, err
	}
	return job, nil
}

func (d *impl) Pop(ctx context.Context, queue string) (qdriver.Job, error) {
	queue = d.resolveQueue(queue)
	for {
		d.mu.Lock()
		if d.closed {
			d.mu.Unlock()
			return nil, qdriver.ErrClosed
		}
		d.mu.Unlock()

		if job, ok, err := d.popDelayed(ctx, queue); err != nil {
			return nil, err
		} else if ok {
			return job, nil
		}

		res, err := d.client.BLPop(ctx, time.Second, d.listKey(queue)).Result()
		if err == goredis.Nil {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
				continue
			}
		}
		if err != nil {
			return nil, err
		}
		if len(res) < 2 {
			continue
		}
		var data storedJob
		if err := json.Unmarshal([]byte(res[1]), &data); err != nil {
			return nil, fmt.Errorf("queue/redis: decode job: %w", err)
		}
		return &redisJob{data: data}, nil
	}
}

func (d *impl) popDelayed(ctx context.Context, queue string) (qdriver.Job, bool, error) {
	now := float64(time.Now().UnixMilli())
	members, err := d.client.ZRangeByScore(ctx, d.delayedKey(queue), &goredis.ZRangeBy{
		Min: "-inf",
		Max: fmt.Sprintf("%f", now),
		Count: 1,
	}).Result()
	if err != nil {
		return nil, false, err
	}
	if len(members) == 0 {
		return nil, false, nil
	}
	removed, err := d.client.ZRem(ctx, d.delayedKey(queue), members[0]).Result()
	if err != nil {
		return nil, false, err
	}
	if removed == 0 {
		return nil, false, nil
	}
	var data storedJob
	if err := json.Unmarshal([]byte(members[0]), &data); err != nil {
		return nil, false, fmt.Errorf("queue/redis: decode delayed job: %w", err)
	}
	return &redisJob{data: data}, true, nil
}

func (d *impl) Delete(context.Context, qdriver.Job) error {
	return nil
}

func (d *impl) Release(ctx context.Context, job qdriver.Job, delay time.Duration) error {
	rj, ok := job.(*redisJob)
	if !ok {
		return fmt.Errorf("queue/redis: unknown job type %T", job)
	}
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return qdriver.ErrClosed
	}
	d.mu.Unlock()

	rj.data.Attempts++
	raw, err := json.Marshal(rj.data)
	if err != nil {
		return err
	}
	if delay <= 0 {
		return d.client.RPush(ctx, d.listKey(rj.data.Queue), raw).Err()
	}
	runAt := time.Now().Add(delay)
	return d.client.ZAdd(ctx, d.delayedKey(rj.data.Queue), goredis.Z{
		Score:  float64(runAt.UnixMilli()),
		Member: raw,
	}).Err()
}

func (d *impl) Shutdown(_ context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return nil
	}
	d.closed = true
	return d.client.Close()
}

func buildOptions(spec qdriver.Spec) (*goredis.Options, error) {
	if spec.URL != "" {
		opts, err := goredis.ParseURL(spec.URL)
		if err != nil {
			return nil, fmt.Errorf("queue/redis: parse URL: %w", err)
		}
		return opts, nil
	}
	if spec.Address == "" {
		return nil, fmt.Errorf("queue/redis: URL or ADDRESS is required")
	}
	return &goredis.Options{
		Addr:     spec.Address,
		Username: spec.Username,
		Password: spec.Password,
		DB:       spec.DB,
	}, nil
}

// RedisKey returns the prefixed Redis key (exported for tests).
func RedisKey(prefix, suffix string) string {
	return prefix + suffix
}
