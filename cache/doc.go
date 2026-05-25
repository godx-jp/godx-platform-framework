// Package cache is a Laravel-faithful, multi-store key/value cache for
// the godx platform framework. Like Laravel's Cache facade, the
// application reaches for a named Store and calls high-level methods
// (Get, Put, Remember, Increment, Pull, …); the actual backend
// (memory, file, Redis, …) is selected once at deploy time by
// configuration.
//
// Architecture
//
//	Manager  ─ holds named Stores; one Store per backend
//	  └─ Store(name) ─ user-facing handle with Laravel-style methods
//	        └─ driver.Driver ─ thin contract every backend implements
//
// Built-in drivers
//
//   - memory  — in-process map, periodic TTL sweeper. Light. Always
//     compiled in.
//   - file    — on-disk, one *.cache file per key with a JSON envelope
//     containing the value and expiry. Mirrors Laravel's FileStore
//     layout. Light. Always compiled in.
//   - redis   — go-redis/v9 client. Heavy — opt in via
//     `import _ "github.com/godx-jp/godx-platform-framework/cache/drivers/redis"`.
//
// Database-backed cache (MySQL/Postgres/SQLite) is intentionally out
// of scope; mounting a relational table as a key/value cache has poor
// operational ergonomics and there is no shortage of better backends.
//
// Quick start
//
//	import (
//	    "github.com/godx-jp/godx-platform-framework/cache"
//	    "github.com/godx-jp/godx-platform-framework/framework"
//	)
//
//	app := framework.New("svc", "1.0.0").Use(cache.Module)
//	if err := app.Init(ctx); err != nil { return err }
//
//	mgr, _ := cache.FromApp(app)
//	store := mgr.Default()
//	_ = store.Put(ctx, "answer", []byte("42"), 30*time.Minute)
//	val, ok, _ := store.Get(ctx, "answer")
//
// With nothing in the environment you get a single in-memory store
// named "memory". Configure stores via the CACHE_STORE_<NAME>_*
// env-var family — see docs/CONFIGURATION.md § Cache.
package cache
