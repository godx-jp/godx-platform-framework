// Package resilient wraps the stdlib driver with retry, exponential
// backoff, and a simple circuit breaker via the resilience module.
package resilient

import (
	"context"
	"errors"
	"net/http"
	"time"

	hdriver "github.com/godx-jp/godx-platform-framework/httpclient/driver"
	"github.com/godx-jp/godx-platform-framework/httpclient/drivers/stdlib"
	"github.com/godx-jp/godx-platform-framework/resilience"
)

func init() {
	hdriver.Register(hdriver.DriverResilient, func(ctx context.Context, spec hdriver.Spec) (hdriver.Client, error) {
		return New(ctx, spec)
	})
}

type client struct {
	inner hdriver.Client
	retry resilience.RetryConfig
	cb    *resilience.CircuitBreaker
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
		inner: inner,
		retry: resilience.RetryConfig{
			MaxAttempts: retries + 1,
			Backoff:     backoff,
			Jitter:      true,
			Retryable:   retryable,
		},
		cb: resilience.NewCircuitBreaker(resilience.CircuitBreakerConfig{
			MaxFailures:  maxFail,
			ResetTimeout: reset,
		}),
	}, nil
}

func (c *client) Name() string { return hdriver.DriverResilient }

func (c *client) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	if err := c.cb.Allow(); err != nil {
		return nil, hdriver.ErrCircuitOpen
	}
	var resp *http.Response
	err := resilience.Do(ctx, c.retry, func(ctx context.Context) error {
		r, err := c.inner.Do(ctx, req)
		if err != nil {
			c.cb.Failure()
			return err
		}
		if r.StatusCode >= 500 && idempotent(req.Method) {
			c.cb.Failure()
			st := r.Status
			r.Body.Close()
			return errors.New(st)
		}
		c.cb.Success()
		resp = r
		return nil
	})
	if err != nil {
		if errors.Is(err, resilience.ErrOpen) {
			return nil, hdriver.ErrCircuitOpen
		}
		return nil, err
	}
	return resp, nil
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
	return err != nil && !errors.Is(err, hdriver.ErrCircuitOpen) && !errors.Is(err, resilience.ErrOpen)
}
