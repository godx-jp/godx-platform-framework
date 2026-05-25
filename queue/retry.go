package queue

import (
	"math/rand"
	"time"
)

type BackoffConfig struct {
	Base time.Duration
	Max  time.Duration
}

type RetryPolicy struct {
	MaxAttempts int
	Backoff     BackoffConfig
	DLQSuffix   string
}

func defaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts: 3,
		Backoff: BackoffConfig{
			Base: time.Second,
			Max:  5 * time.Minute,
		},
		DLQSuffix: "-dlq",
	}
}

func (p RetryPolicy) withDefaults() RetryPolicy {
	if p.MaxAttempts <= 0 {
		p.MaxAttempts = 3
	}
	if p.Backoff.Base <= 0 {
		p.Backoff.Base = time.Second
	}
	if p.Backoff.Max <= 0 {
		p.Backoff.Max = 5 * time.Minute
	}
	if p.DLQSuffix == "" {
		p.DLQSuffix = "-dlq"
	}
	return p
}

func computeBackoff(p RetryPolicy, attempt int) time.Duration {
	p = p.withDefaults()
	if attempt <= 0 {
		attempt = 1
	}
	d := p.Backoff.Base * time.Duration(1<<min(attempt-1, 10))
	if d > p.Backoff.Max {
		d = p.Backoff.Max
	}
	jitter := time.Duration(rand.Int63n(int64(d / 10)))
	return d + jitter
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func dlqName(queue string, suffix string) string {
	if suffix == "" {
		suffix = "-dlq"
	}
	return queue + suffix
}
