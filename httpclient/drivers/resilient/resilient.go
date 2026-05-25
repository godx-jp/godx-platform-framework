// Package resilient wraps the stdlib driver with retry, exponential
// backoff, and a simple circuit breaker. Inline primitives — refactored
// to use resilience/ module in v0.10.4.
package resilient

import (
	"context"
	"errors"
	"math/rand"
	"net/http"
	"sync"
	"time"

	hdriver "github.com/godx-jp/godx-platform-framework/httpclient/driver"
	"github.com/godx-jp/godx-platform-framework/httpclient/drivers/stdlib"
)

func init() {
	hdriver.Register(hdriver.DriverResilient, func(ctx context.Context, spec hdriver.Spec) (hdriver.Client, error) {
		return New(ctx, spec)
	})
}

type breaker struct {
	mu           sync.Mutex
	failures     int
	maxFailures  int
	resetTimeout time.Duration
	openUntil    time.Time
}

func (b *breaker) allow() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if time.Now().Before(b.openUntil) {
		return hdriver.ErrCircuitOpen
	}
	return nil
}

func (b *breaker) success() {
	b.mu.Lock()
	b.failures = 0
	b.openUntil = time.Time{}
	b.mu.Unlock()
}

func (b *breaker) failure() {
	b.mu.Lock()
	b.failures++
	if b.failures >= b.maxFailures {
		b.openUntil = time.Now().Add(b.resetTimeout)
	}
	b.mu.Unlock()
}

type client struct {
	inner      hdriver.Client
	maxRetries int
	backoff    time.Duration
	cb         breaker
}

func New(ctx context.Context, spec hdriver.Spec) (hdriver.Client, error) {
	inner, err := stdlib.New(spec)
	if err != nil {
		return nil, err
	}
	retries := spec.MaxRetries
	if retries <= 0 {
		retries = 3
	}
	backoff := spec.RetryBackoff
	if backoff <= 0 {
		backoff = 100 * time.Millisecond
	}
	maxFail := spec.CBMaxFailures
	if maxFail <= 0 {
		maxFail = 5
	}
	reset := spec.CBResetTimeout
	if reset <= 0 {
		reset = 30 * time.Second
	}
	return &client{
		inner:      inner,
		maxRetries: retries,
		backoff:    backoff,
		cb: breaker{
			maxFailures:  maxFail,
			resetTimeout: reset,
		},
	}, nil
}

func (c *client) Name() string { return hdriver.DriverResilient }

func (c *client) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	if err := c.cb.allow(); err != nil {
		return nil, err
	}
	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			jitter := time.Duration(rand.Int63n(int64(c.backoff)))
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(c.backoff + jitter):
			}
		}
		resp, err := c.inner.Do(ctx, req)
		if err != nil {
			lastErr = err
			c.cb.failure()
			if !retryable(err) {
				return nil, err
			}
			continue
		}
		if resp.StatusCode >= 500 && idempotent(req.Method) {
			lastErr = errors.New(resp.Status)
			resp.Body.Close()
			c.cb.failure()
			continue
		}
		c.cb.success()
		return resp, nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, hdriver.ErrCircuitOpen
}

func (c *client) Shutdown(ctx context.Context) error { return c.inner.Shutdown(ctx) }

func idempotent(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPut, http.MethodDelete, http.MethodOptions:
		return true
	default:
		return false
	}
}

func retryable(err error) bool {
	return err != nil && !errors.Is(err, hdriver.ErrCircuitOpen)
}
