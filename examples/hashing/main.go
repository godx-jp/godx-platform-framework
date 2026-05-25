// Run with `go run ./examples/hashing` from the repo root.
//
// Pick a driver via env:
//
//	HASHING_DEFAULT=argon2id go run ./examples/hashing
//	HASHING_DEFAULT=scrypt   go run ./examples/hashing
//	HASHING_DEFAULT=bcrypt   HASHING_BCRYPT_COST=12 go run ./examples/hashing
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/godx-jp/godx-platform-framework/framework"
	"github.com/godx-jp/godx-platform-framework/hashing"
)

func main() {
	ctx := context.Background()
	app := framework.New("hashing-example", "0.0.0").Use(hashing.Module)
	if err := app.Init(ctx); err != nil {
		log.Fatalf("init: %v", err)
	}
	defer func() { _ = app.Shutdown(ctx) }()

	mgr, err := hashing.FromApp(app)
	if err != nil {
		log.Fatalf("hashing not wired: %v", err)
	}
	h := mgr.Default()
	fmt.Printf("driver: %s\n", h.Name())

	enc, err := h.Make(ctx, "hunter2")
	if err != nil {
		log.Fatalf("Make: %v", err)
	}
	fmt.Printf("encoded: %s\n", enc)

	ok, _ := h.Check(ctx, "hunter2", enc)
	fmt.Printf("Check correct: %v\n", ok)
	ok, _ = h.Check(ctx, "wrong", enc)
	fmt.Printf("Check wrong:   %v\n", ok)

	info, _ := h.Info(enc)
	fmt.Printf("Info:    algorithm=%s params=%v\n", info.Algorithm, info.Params)
	fmt.Printf("Rehash:  %v\n", h.NeedsRehash(enc))
}
