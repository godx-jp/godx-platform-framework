package lock

import (
	"context"
	"testing"
	"time"
)

func TestMemoryTryAcquire(t *testing.T) {
	m := NewMemory()
	release, ok, err := m.TryAcquire(context.Background(), "job")
	if err != nil || !ok || release == nil {
		t.Fatalf("first acquire failed ok=%v err=%v", ok, err)
	}
	_, ok2, _ := m.TryAcquire(context.Background(), "job")
	if ok2 {
		t.Fatalf("second acquire should fail")
	}
	if err := release(); err != nil {
		t.Fatalf("release: %v", err)
	}
	_, ok3, _ := m.TryAcquire(context.Background(), "job")
	if !ok3 {
		t.Fatalf("acquire after release should succeed")
	}
}

func TestCacheRequiresStore(t *testing.T) {
	if _, err := NewCache(CacheOptions{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCacheDistributedLock(t *testing.T) {
	mc := &memStore{data: map[string][]byte{}}
	c, err := NewCache(CacheOptions{Store: mc, Owner: "host-a", TTL: time.Minute})
	if err != nil {
		t.Fatalf("NewCache: %v", err)
	}
	release, ok, err := c.TryAcquire(context.Background(), "nightly")
	if err != nil || !ok {
		t.Fatalf("acquire: ok=%v err=%v", ok, err)
	}
	_, ok2, _ := c.TryAcquire(context.Background(), "nightly")
	if ok2 {
		t.Fatalf("second holder should be rejected")
	}
	if err := release(); err != nil {
		t.Fatalf("release: %v", err)
	}
}

type memStore struct {
	data map[string][]byte
}

func (m *memStore) Add(_ context.Context, key string, value []byte, _ time.Duration) (bool, error) {
	if _, ok := m.data[key]; ok {
		return false, nil
	}
	m.data[key] = value
	return true, nil
}

func (m *memStore) Forget(_ context.Context, key string) error {
	delete(m.data, key)
	return nil
}
