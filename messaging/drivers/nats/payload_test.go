package nats

import "testing"

// TestPayloadTooLarge verifies the inbound payload-size guard used by the
// Subscribe callback. SECURITY: an oversized body must be flagged so the
// callback drops it instead of copying it into an unbounded buffer.
func TestPayloadTooLarge(t *testing.T) {
	const limit = int64(1 << 20) // 1 MiB

	cases := []struct {
		name  string
		size  int
		limit int64
		want  bool
	}{
		{"normal under limit", 1024, limit, false},
		{"exactly at limit", int(limit), limit, false},
		{"one over limit", int(limit) + 1, limit, true},
		{"way over limit", int(limit) * 4, limit, true},
		{"zero limit disables check", int(limit) * 4, 0, false},
		{"negative limit disables check", int(limit) * 4, -1, false},
		{"empty body always ok", 0, limit, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := payloadTooLarge(tc.size, tc.limit); got != tc.want {
				t.Fatalf("payloadTooLarge(%d, %d) = %v, want %v", tc.size, tc.limit, got, tc.want)
			}
		})
	}
}

// TestExtraInt64 verifies the per-connection override parsing used to populate
// broker.maxPayload from Spec.Extra["max_payload_bytes"].
func TestExtraInt64(t *testing.T) {
	const def = int64(1 << 20)

	cases := []struct {
		name  string
		extra map[string]string
		want  int64
	}{
		{"nil map uses default", nil, def},
		{"missing key uses default", map[string]string{"other": "5"}, def},
		{"blank value uses default", map[string]string{"max_payload_bytes": "  "}, def},
		{"invalid value uses default", map[string]string{"max_payload_bytes": "abc"}, def},
		{"valid override", map[string]string{"max_payload_bytes": "2048"}, 2048},
		{"zero disables check", map[string]string{"max_payload_bytes": "0"}, 0},
		{"negative disables check", map[string]string{"max_payload_bytes": "-1"}, -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := extraInt64(tc.extra, "max_payload_bytes", def); got != tc.want {
				t.Fatalf("extraInt64 = %d, want %d", got, tc.want)
			}
		})
	}
}
