package messaging_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/godx-jp/godx-platform-framework/messaging"
	"github.com/godx-jp/godx-platform-framework/messaging/driver"
	"github.com/godx-jp/godx-platform-framework/messaging/envelope"
	_ "github.com/godx-jp/godx-platform-framework/messaging/drivers/memory"
)

func TestMemoryPublishSubscribe(t *testing.T) {
	ctx := context.Background()
	b, err := driver.New(ctx, driver.Spec{Name: driver.DriverMemory})
	if err != nil {
		t.Fatal(err)
	}
	mgr := messaging.NewManager()
	if err := mgr.Add("platform", b); err != nil {
		t.Fatal(err)
	}
	_ = mgr.SetDefault("platform")
	pub, err := mgr.Publisher()
	if err != nil {
		t.Fatal(err)
	}
	sub, err := mgr.Subscriber()
	if err != nil {
		t.Fatal(err)
	}
	var count atomic.Int32
	_, err = sub.Subscribe(ctx, "orders.>", func(_ context.Context, e envelope.Event) error {
		count.Add(1)
		if e.Type != "orders.order.placed.v1" {
			t.Fatalf("type=%q", e.Type)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := pub.Publish(ctx, envelope.Event{
		ID: "1", Source: "orders", Type: "orders.order.placed.v1", Data: []byte(`{}`),
	}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	if count.Load() != 1 {
		t.Fatalf("count=%d", count.Load())
	}
}

func TestCloudEventsValidate(t *testing.T) {
	if err := (envelope.Event{}).Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}
