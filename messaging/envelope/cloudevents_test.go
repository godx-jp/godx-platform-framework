package envelope_test

import (
	"testing"

	"github.com/godx-jp/godx-platform-framework/messaging/envelope"
)

func TestEncodeDecodeRoundtrip(t *testing.T) {
	e := envelope.Event{
		ID: "abc", Source: "svc", Type: "orders.order.placed.v1", Data: []byte(`{"ok":true}`),
	}
	b, err := envelope.Encode(e)
	if err != nil {
		t.Fatal(err)
	}
	got, err := envelope.Decode(b)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != e.ID || got.Type != e.Type {
		t.Fatalf("got=%+v", got)
	}
}
