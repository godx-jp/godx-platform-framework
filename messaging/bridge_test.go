package messaging_test

import (
	"context"
	"sync"
	"testing"

	"github.com/godx-jp/godx-platform-framework/events"
	"github.com/godx-jp/godx-platform-framework/messaging"
	"github.com/godx-jp/godx-platform-framework/messaging/envelope"
	mdriver "github.com/godx-jp/godx-platform-framework/messaging/driver"
	memdrv "github.com/godx-jp/godx-platform-framework/messaging/drivers/memory"
)

func TestWireBridgePublishesMatchingEvents(t *testing.T) {
	broker, err := memdrv.New(context.Background(), mdriver.Spec{})
	if err != nil {
		t.Fatal(err)
	}
	defer broker.Close(context.Background())

	mgr := messaging.NewManager()
	if err := mgr.Add("default", broker); err != nil {
		t.Fatal(err)
	}
	pub, err := mgr.Publisher()
	if err != nil {
		t.Fatal(err)
	}
	sub, err := mgr.Subscriber()
	if err != nil {
		t.Fatal(err)
	}

	var got envelope.Event
	var wg sync.WaitGroup
	wg.Add(1)
	_, err = sub.Subscribe(context.Background(), "integration.>", func(_ context.Context, e envelope.Event) error {
		got = e
		wg.Done()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	bus := events.New()
	br, err := messaging.WireBridge(context.Background(), bus, pub, messaging.BridgeOptions{
		Prefix: "integration.",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer br.Close()

	if err := bus.Dispatch(context.Background(), events.Event{
		Name:    "integration.order.created",
		Payload: []byte(`{"id":1}`),
	}); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	if got.Type != "com.godx.integration.order.created" {
		t.Fatalf("type=%q", got.Type)
	}
}
