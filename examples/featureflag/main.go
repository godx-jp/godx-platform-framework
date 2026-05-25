// Run with `go run ./examples/featureflag` from the repo root.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/godx-jp/godx-platform-framework/config"
	"github.com/godx-jp/godx-platform-framework/featureflag"
	"github.com/godx-jp/godx-platform-framework/framework"
)

func main() {
	ctx := context.Background()
	repo := config.NewRepository(map[string]any{
		"flags": map[string]any{
			"new-checkout": true,
			"beta-ui": map[string]any{
				"users": []any{"alice@example.com"},
			},
		},
	})
	cfg := featureflag.Config{Driver: "config", Prefix: "flags", Cache: true}
	app := framework.New("featureflag-example", "0.0.0").
		Use(featureflag.ModuleWithConfig(cfg, repo))
	if err := app.Init(ctx); err != nil {
		log.Fatalf("init: %v", err)
	}
	defer func() { _ = app.Shutdown(ctx) }()

	eval, err := featureflag.FromApp(app)
	if err != nil {
		log.Fatalf("evaluator: %v", err)
	}

	for _, tc := range []struct {
		flag, user string
		want       bool
	}{
		{"new-checkout", "anyone", true},
		{"beta-ui", "alice@example.com", true},
		{"beta-ui", "bob@example.com", false},
	} {
		ok, err := eval.Enabled(ctx, tc.flag, tc.user, nil)
		if err != nil {
			log.Fatalf("Enabled(%q): %v", tc.flag, err)
		}
		fmt.Printf("flag=%-14s user=%-22s enabled=%v (want %v)\n", tc.flag, tc.user, ok, tc.want)
	}
}
