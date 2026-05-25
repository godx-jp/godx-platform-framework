// Package ratelimit provides token-bucket rate limiting with swappable
// drivers (memory, redis) and HTTP middleware that returns 429 +
// Retry-After when a key is over quota.
//
//	app := framework.New("svc", "1.0.0").Use(ratelimit.Module)
//	_ = app.Init(ctx)
//	mgr, _ := ratelimit.FromApp(app)
//	ok, _ := mgr.Allow(ctx, "user:42")
//
// Drivers:
//
//	memory - in-process sync.Map buckets (light, auto)
//	redis  - distributed token bucket via Lua (heavy, blank import)
package ratelimit
