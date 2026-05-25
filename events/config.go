package events

import (
	"os"
	"strconv"
	"strings"
)

// Environment variable names used by the events module.
const (
	// EnvAsync toggles the async wrapper around the in-process bus.
	// Defaults to false (synchronous).
	EnvAsync = "EVENTS_ASYNC"
	// EnvAsyncWorkers controls the worker count for the async wrapper.
	// Defaults to 4 when EVENTS_ASYNC=true.
	EnvAsyncWorkers = "EVENTS_ASYNC_WORKERS"
	// EnvAsyncQueueSize controls the buffered queue depth.
	// Defaults to 256 when EVENTS_ASYNC=true.
	EnvAsyncQueueSize = "EVENTS_ASYNC_QUEUE_SIZE"
)

// Config configures the events module.
type Config struct {
	// Async wraps the bus in the async dispatcher when true.
	Async bool
	// AsyncWorkers is the worker-pool size for the async wrapper.
	// Ignored when Async is false. Defaults to 4.
	AsyncWorkers int
	// AsyncQueueSize is the buffered queue depth for the async
	// wrapper. Ignored when Async is false. Defaults to 256.
	AsyncQueueSize int
}

// LoadConfigFromEnv builds a Config from the process environment.
func LoadConfigFromEnv() Config {
	async := false
	if v := strings.TrimSpace(os.Getenv(EnvAsync)); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			async = b
		}
	}
	workers := 4
	if v := strings.TrimSpace(os.Getenv(EnvAsyncWorkers)); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			workers = n
		}
	}
	qs := 256
	if v := strings.TrimSpace(os.Getenv(EnvAsyncQueueSize)); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			qs = n
		}
	}
	return Config{
		Async:          async,
		AsyncWorkers:   workers,
		AsyncQueueSize: qs,
	}
}
