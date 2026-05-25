// Package memory is an in-process token-bucket limiter keyed by string.
// Each key gets its own bucket stored in a sync.Map. Suitable for
// single-process services and tests.
package memory

import (
	"context"
	"math"
	"sync"
	"time"

	rdriver "github.com/godx-jp/godx-platform-framework/ratelimit/driver"
)

const (
	defaultRate  = 10.0
	defaultBurst = 20
)

func init() {
	rdriver.Register(rdriver.DriverMemory, func(_ context.Context, spec rdriver.Spec) (rdriver.Limiter, error) {
		return New(spec.Rate, spec.Burst), nil
	})
}

type bucket struct {
	mu     sync.Mutex
	tokens float64
	last   time.Time
}

type limiter struct {
	rate   float64
	burst  float64
	buckets sync.Map

	mu     sync.Mutex
	closed bool
}

// New constructs a memory limiter. Non-positive rate defaults to 10/s;
// non-positive burst defaults to 20.
func New(rate float64, burst int) rdriver.Limiter {
	if rate <= 0 {
		rate = defaultRate
	}
	b := float64(burst)
	if b <= 0 {
		b = defaultBurst
	}
	return &limiter{rate: rate, burst: b}
}

func (l *limiter) Name() string { return rdriver.DriverMemory }

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
	b := l.bucket(key)
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	if b.last.IsZero() {
		b.tokens = l.burst
		b.last = now
	} else {
		elapsed := now.Sub(b.last).Seconds()
		b.tokens = math.Min(l.burst, b.tokens+elapsed*l.rate)
		b.last = now
	}
	if b.tokens < 1 {
		return false, nil
	}
	b.tokens--
	return true, nil
}

func (l *limiter) Reset(_ context.Context, key string) {
	if key == "" {
		key = "_"
	}
	l.buckets.Delete(key)
}

func (l *limiter) Shutdown(_ context.Context) error {
	l.mu.Lock()
	l.closed = true
	l.mu.Unlock()
	return nil
}

func (l *limiter) bucket(key string) *bucket {
	v, _ := l.buckets.LoadOrStore(key, &bucket{})
	return v.(*bucket)
}

func (l *limiter) checkOpen() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return rdriver.ErrClosed
	}
	return nil
}
