package featureflag

import (
	"context"
	"testing"
	"time"

	"github.com/godx-jp/godx-platform-framework/config"
	fdriver "github.com/godx-jp/godx-platform-framework/featureflag/driver"
	_ "github.com/godx-jp/godx-platform-framework/featureflag/drivers/config"
)

type stubProvider struct {
	name string
	val  bool
	err  error
	calls int
}

func (s *stubProvider) Name() string { return s.name }
func (s *stubProvider) Enabled(_ context.Context, _, _ string, _ map[string]any) (bool, error) {
	s.calls++
	return s.val, s.err
}
func (s *stubProvider) Shutdown(_ context.Context) error { return nil }

func TestEvaluatorEnabled(t *testing.T) {
	p := &stubProvider{name: "test", val: true}
	e, err := NewEvaluator(EvaluatorOptions{Provider: p})
	if err != nil {
		t.Fatalf("NewEvaluator: %v", err)
	}
	ok, err := e.Enabled(context.Background(), "x", "u1", nil)
	if err != nil || !ok || p.calls != 1 {
		t.Fatalf("ok=%v err=%v calls=%d", ok, err, p.calls)
	}
}

func TestEvaluatorCache(t *testing.T) {
	p := &stubProvider{name: "test", val: true}
	e, err := NewEvaluator(EvaluatorOptions{Provider: p, CacheEnabled: true, CacheTTL: time.Minute})
	if err != nil {
		t.Fatalf("NewEvaluator: %v", err)
	}
	_, _ = e.Enabled(context.Background(), "x", "u1", nil)
	_, _ = e.Enabled(context.Background(), "x", "u1", nil)
	if p.calls != 1 {
		t.Fatalf("cache should dedupe calls, got %d", p.calls)
	}
}

func TestConfigDriverFromRepository(t *testing.T) {
	repo := config.NewRepository(map[string]any{
		"flags": map[string]any{
			"beta": true,
			"canary": map[string]any{
				"users": []any{"alice", "bob"},
			},
		},
	})
	c := fdriver.Lookup(fdriver.DriverConfig)
	p, err := c(context.Background(), fdriver.Spec{Name: fdriver.DriverConfig, Repo: repo, Prefix: "flags"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ok, err := p.Enabled(context.Background(), "beta", "", nil)
	if err != nil || !ok {
		t.Fatalf("beta: ok=%v err=%v", ok, err)
	}
	ok, err = p.Enabled(context.Background(), "canary", "alice", nil)
	if err != nil || !ok {
		t.Fatalf("canary alice: ok=%v err=%v", ok, err)
	}
	ok, err = p.Enabled(context.Background(), "canary", "eve", nil)
	if err != nil || ok {
		t.Fatalf("canary eve: ok=%v err=%v", ok, err)
	}
}
