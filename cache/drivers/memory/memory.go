// Package memory provides an in-process, goroutine-safe cache driver
// backed by a map. Useful for tests, single-process services, and as
// the default zero-config store.
//
// Light driver — no third-party dependencies; auto-registers under the
// name "memory" via the parent cache.register import.
package memory

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	cdriver "github.com/godx-jp/godx-platform-framework/cache/driver"
)

func init() {
	cdriver.Register(cdriver.DriverMemory, construct)
}

// Name is exported so callers can reference the driver name without
// hard-coding a string.
const Name = cdriver.DriverMemory

func construct(_ context.Context, spec cdriver.Spec) (cdriver.Driver, error) {
	d := &impl{
		prefix: spec.Prefix,
		items:  make(map[string]entry),
		done:   make(chan struct{}),
	}
	d.wg.Add(1)
	go d.sweep()
	return d, nil
}

type entry struct {
	val       []byte
	expiresAt time.Time // zero = no expiry
}

func (e entry) expired(now time.Time) bool {
	return !e.expiresAt.IsZero() && !now.Before(e.expiresAt)
}

type impl struct {
	prefix string

	mu     sync.Mutex
	items  map[string]entry
	closed bool

	done     chan struct{}
	wg       sync.WaitGroup
	stopOnce sync.Once
}

func (d *impl) full(key string) string { return d.prefix + key }

// sweep periodically purges expired entries so memory does not grow
// unbounded under workloads that write keys with TTLs but never read
// them again.
func (d *impl) sweep() {
	defer d.wg.Done()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-d.done:
			return
		case now := <-ticker.C:
			d.mu.Lock()
			for k, e := range d.items {
				if e.expired(now) {
					delete(d.items, k)
				}
			}
			d.mu.Unlock()
		}
	}
}

func (d *impl) Get(_ context.Context, key string) ([]byte, bool, error) {
	k := d.full(key)
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return nil, false, cdriver.ErrClosed
	}
	e, ok := d.items[k]
	if !ok {
		return nil, false, nil
	}
	if e.expired(time.Now()) {
		delete(d.items, k)
		return nil, false, nil
	}
	return append([]byte(nil), e.val...), true, nil
}

func (d *impl) Put(_ context.Context, key string, val []byte, ttl time.Duration) error {
	k := d.full(key)
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return cdriver.ErrClosed
	}
	d.items[k] = newEntry(val, ttl)
	return nil
}

func newEntry(val []byte, ttl time.Duration) entry {
	e := entry{val: append([]byte(nil), val...)}
	if ttl > 0 {
		e.expiresAt = time.Now().Add(ttl)
	}
	return e
}

func (d *impl) Add(_ context.Context, key string, val []byte, ttl time.Duration) (bool, error) {
	k := d.full(key)
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return false, cdriver.ErrClosed
	}
	if e, ok := d.items[k]; ok && !e.expired(time.Now()) {
		return false, nil
	}
	d.items[k] = newEntry(val, ttl)
	return true, nil
}

func (d *impl) Forget(_ context.Context, key string) error {
	k := d.full(key)
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return cdriver.ErrClosed
	}
	delete(d.items, k)
	return nil
}

func (d *impl) Has(_ context.Context, key string) (bool, error) {
	k := d.full(key)
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return false, cdriver.ErrClosed
	}
	e, ok := d.items[k]
	if !ok {
		return false, nil
	}
	if e.expired(time.Now()) {
		delete(d.items, k)
		return false, nil
	}
	return true, nil
}

// Flush removes every key with the configured prefix. When no prefix
// is set, the entire store is wiped.
func (d *impl) Flush(_ context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return cdriver.ErrClosed
	}
	if d.prefix == "" {
		d.items = make(map[string]entry)
		return nil
	}
	for k := range d.items {
		if strings.HasPrefix(k, d.prefix) {
			delete(d.items, k)
		}
	}
	return nil
}

func (d *impl) Increment(_ context.Context, key string, delta int64) (int64, error) {
	return d.adjust(key, delta)
}

func (d *impl) Decrement(_ context.Context, key string, delta int64) (int64, error) {
	return d.adjust(key, -delta)
}

func (d *impl) adjust(key string, delta int64) (int64, error) {
	k := d.full(key)
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return 0, cdriver.ErrClosed
	}
	now := time.Now()
	var current int64
	var expiresAt time.Time
	if e, ok := d.items[k]; ok && !e.expired(now) {
		n, err := strconv.ParseInt(string(e.val), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("%w: key=%q stored=%q", cdriver.ErrNotInteger, key, e.val)
		}
		current = n
		expiresAt = e.expiresAt
	}
	current += delta
	d.items[k] = entry{
		val:       []byte(strconv.FormatInt(current, 10)),
		expiresAt: expiresAt,
	}
	return current, nil
}

func (d *impl) Shutdown(_ context.Context) error {
	d.stopOnce.Do(func() { close(d.done) })
	d.wg.Wait()
	d.mu.Lock()
	d.closed = true
	d.items = nil
	d.mu.Unlock()
	return nil
}
