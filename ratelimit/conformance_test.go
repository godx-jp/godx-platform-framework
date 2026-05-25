package ratelimit

import (
	"context"
	"errors"
	"testing"

	rdriver "github.com/godx-jp/godx-platform-framework/ratelimit/driver"
	memdrv "github.com/godx-jp/godx-platform-framework/ratelimit/drivers/memory"
)

type driverCase struct {
	name  string
	build func(t *testing.T) rdriver.Limiter
	rate  float64
	burst int
}

func memoryCase() driverCase {
	return driverCase{
		name:  rdriver.DriverMemory,
		rate:  5,
		burst: 2,
		build: func(t *testing.T) rdriver.Limiter {
			return memdrv.New(5, 2)
		},
	}
}

func runConformance(t *testing.T, dc driverCase) {
	t.Run(dc.name, func(t *testing.T) {
		t.Run("name_matches_driver", func(t *testing.T) {
			l := dc.build(t)
			defer l.Shutdown(context.Background())
			if l.Name() != dc.name {
				t.Fatalf("Name=%q want %q", l.Name(), dc.name)
			}
		})

		t.Run("allow_within_burst", func(t *testing.T) {
			l := dc.build(t)
			defer l.Shutdown(context.Background())
			ctx := context.Background()
			key := "conform-burst"
			for i := 0; i < dc.burst; i++ {
				ok, err := l.Allow(ctx, key)
				if err != nil {
					t.Fatalf("Allow: %v", err)
				}
				if !ok {
					t.Fatalf("request %d denied inside burst", i+1)
				}
			}
		})

		t.Run("deny_over_burst", func(t *testing.T) {
			l := dc.build(t)
			defer l.Shutdown(context.Background())
			ctx := context.Background()
			key := "conform-over"
			for i := 0; i < dc.burst; i++ {
				_, _ = l.Allow(ctx, key)
			}
			ok, err := l.Allow(ctx, key)
			if err != nil {
				t.Fatalf("Allow: %v", err)
			}
			if ok {
				t.Fatalf("expected denial after burst exhausted")
			}
		})

		t.Run("reset_restores_quota", func(t *testing.T) {
			l := dc.build(t)
			defer l.Shutdown(context.Background())
			ctx := context.Background()
			key := "conform-reset"
			for i := 0; i < dc.burst; i++ {
				_, _ = l.Allow(ctx, key)
			}
			ok, _ := l.Allow(ctx, key)
			if ok {
				t.Fatalf("expected denial before reset")
			}
			l.Reset(ctx, key)
			ok, err := l.Allow(ctx, key)
			if err != nil || !ok {
				t.Fatalf("after reset: ok=%v err=%v", ok, err)
			}
		})

		t.Run("keys_are_isolated", func(t *testing.T) {
			l := dc.build(t)
			defer l.Shutdown(context.Background())
			ctx := context.Background()
			for i := 0; i < dc.burst; i++ {
				_, _ = l.Allow(ctx, "a")
			}
			ok, _ := l.Allow(ctx, "a")
			if ok {
				t.Fatalf("key a should be exhausted")
			}
			ok, err := l.Allow(ctx, "b")
			if err != nil || !ok {
				t.Fatalf("key b should still have quota: ok=%v err=%v", ok, err)
			}
		})

		t.Run("shutdown_is_idempotent_and_blocks_ops", func(t *testing.T) {
			l := dc.build(t)
			if err := l.Shutdown(context.Background()); err != nil {
				t.Fatalf("first: %v", err)
			}
			if err := l.Shutdown(context.Background()); err != nil {
				t.Fatalf("second: %v", err)
			}
			_, err := l.Allow(context.Background(), "k")
			if !errors.Is(err, rdriver.ErrClosed) {
				t.Fatalf("Allow after Shutdown err=%v", err)
			}
		})
	})
}

func TestConformance(t *testing.T) {
	runConformance(t, memoryCase())
}

func TestLightDriversAutoRegister(t *testing.T) {
	if rdriver.Lookup(rdriver.DriverMemory) == nil {
		t.Fatalf("memory driver not auto-registered")
	}
}
