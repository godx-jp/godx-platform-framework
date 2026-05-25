// Run with `go run ./examples/pipeline` from the repo root.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/godx-jp/godx-platform-framework/pipeline"
)

type Order struct {
	Total int
	Notes []string
}

func main() {
	ctx := context.Background()

	applyCoupon := func(ctx context.Context, o *Order, next pipeline.Next[*Order]) (*Order, error) {
		o.Total -= 100
		o.Notes = append(o.Notes, "coupon: -100")
		return next(ctx, o)
	}
	applyVAT := func(ctx context.Context, o *Order, next pipeline.Next[*Order]) (*Order, error) {
		o.Total = o.Total * 110 / 100
		o.Notes = append(o.Notes, "vat: +10%")
		return next(ctx, o)
	}
	logStage := pipeline.FuncStage(func(ctx context.Context, o *Order) {
		fmt.Printf("[trace] total=%d notes=%v\n", o.Total, o.Notes)
	})

	pipe := pipeline.New[*Order]().
		Through(logStage, applyCoupon, logStage, applyVAT, logStage).
		Then(func(ctx context.Context, o *Order) (*Order, error) {
			o.Notes = append(o.Notes, "checkout")
			return o, nil
		})

	final, err := pipe(ctx, &Order{Total: 1000})
	if err != nil {
		log.Fatalf("pipe: %v", err)
	}
	fmt.Printf("final order: total=%d notes=%v\n", final.Total, final.Notes)
}
