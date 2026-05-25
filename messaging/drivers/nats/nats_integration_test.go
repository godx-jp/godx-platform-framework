//go:build integration

package nats_test

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	mdriver "github.com/godx-jp/godx-platform-framework/messaging/driver"
	natsdrv "github.com/godx-jp/godx-platform-framework/messaging/drivers/nats"
	"github.com/godx-jp/godx-platform-framework/messaging/envelope"
)

func TestJetStreamPublishSubscribeAck(t *testing.T) {
	url := os.Getenv("NATS_URL")
	if url == "" {
		url = "nats://127.0.0.1:4222"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	b, err := natsdrv.New(ctx, mdriver.Spec{
		Name:          mdriver.DriverNATS,
		NATSURL:       url,
		StreamName:    "godx_test",
		SubjectPrefix: "",
	})
	if err != nil {
		t.Skipf("NATS unavailable: %v", err)
	}
	defer func() { _ = b.Close(ctx) }()

	subject := "test.integration.ack.v1"
	var wg sync.WaitGroup
	wg.Add(1)
	_, err = b.Subscribe(ctx, subject, func(c context.Context, msg mdriver.Message) error {
		defer wg.Done()
		e, err := envelope.Decode(msg.Body)
		if err != nil {
			return err
		}
		if e.ID != "evt-1" {
			t.Errorf("id=%q", e.ID)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	body, err := envelope.Encode(envelope.Event{
		ID: "evt-1", Source: "test://nats", Type: subject, Data: []byte(`{"ok":true}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Publish(ctx, subject, mdriver.Message{Body: body}); err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for message")
	}
}
