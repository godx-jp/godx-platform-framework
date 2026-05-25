// Package resilient wraps the stdlib driver with retry, exponential
// backoff, and a simple circuit breaker via the resilience module.
package resilient

import (
	"context"
	"errors"
	"fmt"
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

// errNoRetry wraps a failure that must count against the circuit
// breaker but must not trigger a retry attempt (e.g. a 5xx response or
// a transport error on a non-safe HTTP method).
var errNoRetry = errors.New("httpclient: not retryable")

func (c *client) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	if err := c.cb.Allow(); err != nil {
		return nil, hdriver.ErrCircuitOpen
	}
	// Only safe (RFC 7231) methods may be retried. Retrying a
	// non-safe method (POST/PUT/PATCH/DELETE) can duplicate side
	// effects — both on a 5xx response and on a transport error,
	// since the request may already have reached the upstream.
	safeMethod := safe(req.Method)
	var resp *http.Response
	err := resilience.Do(ctx, c.retry, func(ctx context.Context) error {
		r, err := c.inner.Do(ctx, req)
		if err != nil {
			c.cb.Failure()
			if !safeMethod {
				// Surface the cause but block further attempts.
				return fmt.Errorf("%w: %v", errNoRetry, err)
			}
			return err
		}
		if r.StatusCode >= 500 {
			c.cb.Failure()
			st := r.Status
			r.Body.Close()
			if !safeMethod {
				return fmt.Errorf("%w: %s", errNoRetry, st)
			}
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

// safe reports whether method is an RFC 7231 "safe" method, for which
// a retry cannot duplicate a side effect. These are the only methods
// the resilient driver retries by default.
func safe(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	default:
		return false
	}
}

func retryable(err error) bool {
	return err != nil &&
		!errors.Is(err, errNoRetry) &&
		!errors.Is(err, hdriver.ErrCircuitOpen) &&
		!errors.Is(err, resilience.ErrOpen)
}
