// Run with `go run ./examples/cache` from the repo root.
//
// Zero-config (uses the in-memory store):
//
//	go run ./examples/cache
//
// With a file-backed store, Laravel-faithful path:
//
//	CACHE_DEFAULT_STORE=files \
//	CACHE_STORES=files \
//	CACHE_STORE_FILES_DRIVER=file \
//	CACHE_STORE_FILES_PATH=./.tmp/cache-example \
//	go run ./examples/cache
//
// With Redis (requires the redis blank import below — already present):
//
//	docker run --rm -d --name redis -p 6379:6379 redis:7
//	CACHE_DEFAULT_STORE=redis \
//	CACHE_STORES=redis \
//	CACHE_STORE_REDIS_URL=redis://127.0.0.1:6379/0 \
//	CACHE_PREFIX=example: \
//	go run ./examples/cache
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/godx-jp/godx-platform-framework/cache"
	_ "github.com/godx-jp/godx-platform-framework/cache/drivers/redis" // opt-in heavy driver
	"github.com/godx-jp/godx-platform-framework/framework"
)

type weather struct {
	City string  `json:"city"`
	Temp float64 `json:"temp"`
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := framework.New("cache-example", "0.7.0").Use(cache.Module)
	if err := app.Init(ctx); err != nil {
		log.Fatalf("init: %v", err)
	}
	defer func() { _ = app.Shutdown(ctx) }()

	mgr, err := cache.FromApp(app)
	if err != nil {
		log.Fatal(err)
	}
	store := mgr.Default()
	fmt.Printf("default cache store = %s (registered: %v)\n", store.Name(), mgr.Stores())

	// 1) Put + Get
	_ = store.Put(ctx, "greeting", []byte("hello, world"), 5*time.Minute)
	if v, ok, _ := store.Get(ctx, "greeting"); ok {
		fmt.Printf("greeting = %s\n", v)
	}

	// 2) Remember — compute once, cache for 30 s.
	v, _ := store.Remember(ctx, "expensive", 30*time.Second, func(context.Context) ([]byte, error) {
		fmt.Println("computing expensive value (this prints only on a cache miss)")
		return []byte("42"), nil
	})
	fmt.Printf("expensive (1st call) = %s\n", v)
	v, _ = store.Remember(ctx, "expensive", 30*time.Second, func(context.Context) ([]byte, error) {
		fmt.Println("THIS SHOULD NOT PRINT — cache should be warm")
		return []byte("99"), nil
	})
	fmt.Printf("expensive (2nd call) = %s\n", v)

	// 3) Atomic counter
	for i := 0; i < 3; i++ {
		n, _ := store.Increment(ctx, "visits", 1)
		fmt.Printf("visits after incr #%d = %d\n", i+1, n)
	}

	// 4) JSON helpers
	_ = store.PutJSON(ctx, "weather:tokyo", weather{City: "Tokyo", Temp: 23.7}, 0)
	var w weather
	if ok, _ := store.GetJSON(ctx, "weather:tokyo", &w); ok {
		fmt.Printf("weather = %+v\n", w)
	}

	// 5) Pull — atomic read-and-delete
	_ = store.Put(ctx, "flash", []byte("one-shot"), 0)
	if v, ok, _ := store.Pull(ctx, "flash"); ok {
		fmt.Printf("pulled flash = %s\n", v)
	}
	if _, ok, _ := store.Get(ctx, "flash"); ok {
		fmt.Println("flash still present (bug!)")
	} else {
		fmt.Println("flash is gone after pull — as expected")
	}

	// Tidy up
	_ = store.Flush(ctx)
}
