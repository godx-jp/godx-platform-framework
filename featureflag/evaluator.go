package featureflag

import (
	"context"
	"fmt"
	"sync"
	"time"

	fdriver "github.com/godx-jp/godx-platform-framework/featureflag/driver"
)

type cacheEntry struct {
	value     bool
	expiresAt time.Time
}

// Evaluator evaluates feature flags through a registered Provider.
type Evaluator struct {
	provider fdriver.Provider

	cacheEnabled bool
	cacheTTL     time.Duration
	cacheMu      sync.RWMutex
	cache        map[string]cacheEntry
}

// EvaluatorOptions configures an Evaluator.
type EvaluatorOptions struct {
	Provider     fdriver.Provider
	CacheEnabled bool
	CacheTTL     time.Duration
}

// NewEvaluator constructs an Evaluator.
func NewEvaluator(opts EvaluatorOptions) (*Evaluator, error) {
	if opts.Provider == nil {
		return nil, fmt.Errorf("featureflag: Provider is required")
	}
	ttl := opts.CacheTTL
	if ttl <= 0 {
		ttl = time.Minute
	}
	e := &Evaluator{
		provider:     opts.Provider,
		cacheEnabled: opts.CacheEnabled,
		cacheTTL:     ttl,
	}
	if opts.CacheEnabled {
		e.cache = map[string]cacheEntry{}
	}
	return e, nil
}

// Enabled reports whether flag is on for user with optional attrs.
func (e *Evaluator) Enabled(ctx context.Context, flag, user string, attrs map[string]any) (bool, error) {
	if flag == "" {
		return false, fmt.Errorf("featureflag: flag name is required")
	}
	if e.cacheEnabled {
		key := cacheKey(flag, user, attrs)
		e.cacheMu.RLock()
		if ent, ok := e.cache[key]; ok && time.Now().Before(ent.expiresAt) {
			e.cacheMu.RUnlock()
			return ent.value, nil
		}
		e.cacheMu.RUnlock()
	}
	val, err := e.provider.Enabled(ctx, flag, user, attrs)
	if err != nil {
		return false, err
	}
	if e.cacheEnabled {
		key := cacheKey(flag, user, attrs)
		e.cacheMu.Lock()
		e.cache[key] = cacheEntry{value: val, expiresAt: time.Now().Add(e.cacheTTL)}
		e.cacheMu.Unlock()
	}
	return val, nil
}

// Shutdown closes the underlying provider.
func (e *Evaluator) Shutdown(ctx context.Context) error {
	return e.provider.Shutdown(ctx)
}

func cacheKey(flag, user string, attrs map[string]any) string {
	// Stable enough for in-process cache; attrs ignored when empty.
	if len(attrs) == 0 {
		return flag + "|" + user
	}
	return fmt.Sprintf("%s|%s|%v", flag, user, attrs)
}
