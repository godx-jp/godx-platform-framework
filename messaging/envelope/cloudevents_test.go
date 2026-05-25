package envelope_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

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

func TestConformanceRequiredAttributes(t *testing.T) {
	e := envelope.Event{
		ID:              "550e8400-e29b-41d4-a716-446655440000",
		Source:          "https://example.com/services/order",
		Type:            "com.example.order.placed.v1",
		Time:            time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC),
		DataContentType: "application/json",
		Data:            []byte(`{"order_id":"o-1"}`),
	}
	b, err := envelope.Encode(e)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"specversion", "id", "source", "type", "time", "datacontenttype", "data"} {
		if _, ok := raw[key]; !ok {
			t.Fatalf("missing required attribute %q", key)
		}
	}
	if got := strings.Trim(string(raw["specversion"]), `"`); got != envelope.SpecVersion {
		t.Fatalf("specversion=%q want %q (doc %s)", got, envelope.SpecVersion, envelope.SpecVersionDoc)
	}
}

func TestDecodeRejectsUnsupportedSpecVersion(t *testing.T) {
	_, err := envelope.Decode([]byte(`{"specversion":"0.3","id":"x","source":"s","type":"t"}`))
	if err == nil || !strings.Contains(err.Error(), "specversion") {
		t.Fatalf("err=%v", err)
	}
}
