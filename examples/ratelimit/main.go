// Run: go run ./examples/ratelimit from repo root.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"

	"github.com/godx-jp/godx-platform-framework/framework"
	"github.com/godx-jp/godx-platform-framework/ratelimit"
	rdriver "github.com/godx-jp/godx-platform-framework/ratelimit/driver"
	"github.com/godx-jp/godx-platform-framework/ratelimit/middleware"
)

func main() {
	ctx := context.Background()
	cfg := ratelimit.Config{
		Default: rdriver.DriverMemory,
		Limiters: map[string]ratelimit.LimiterConfig{
			rdriver.DriverMemory: {
				Driver: rdriver.DriverMemory,
				Spec:   rdriver.Spec{Name: rdriver.DriverMemory, Rate: 2, Burst: 2},
			},
		},
	}
	app := framework.New("ratelimit-example", "0.0.0").Use(ratelimit.ModuleWithConfig(cfg))
	if err := app.Init(ctx); err != nil {
		log.Fatalf("init: %v", err)
	}
	defer app.Shutdown(ctx)

	mgr, _ := ratelimit.FromApp(app)
	lim := mgr.Default()

	handler := middleware.Limit(lim, middleware.ByIP)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok")
	}))

	srv := httptest.NewServer(handler)
	defer srv.Close()

	for i := 1; i <= 3; i++ {
		resp, err := http.Get(srv.URL)
		if err != nil {
			log.Fatalf("request %d: %v", i, err)
		}
		resp.Body.Close()
		fmt.Printf("request %d → %d Retry-After=%q\n", i, resp.StatusCode, resp.Header.Get("Retry-After"))
	}

	ok, _ := mgr.Allow(ctx, "manual-key")
	fmt.Printf("direct Allow(manual-key)=%v\n", ok)
}
