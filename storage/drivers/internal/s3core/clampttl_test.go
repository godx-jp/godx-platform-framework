package s3core

import (
	"testing"
	"time"
)

// clampTTL must floor a non-positive expiry to the 15-minute default and
// cap an over-long one (e.g. a misconfigured 1-year) to maxSignedURLTTL so
// a leaked signed URL cannot outlive a week.
func TestClampTTL(t *testing.T) {
	cases := []struct {
		name string
		in   time.Duration
		want time.Duration
	}{
		{"zero floors to default", 0, 15 * time.Minute},
		{"negative floors to default", -time.Hour, 15 * time.Minute},
		{"in-range passes through", time.Hour, time.Hour},
		{"exactly max passes through", maxSignedURLTTL, maxSignedURLTTL},
		{"over-long clamps to max", 365 * 24 * time.Hour, maxSignedURLTTL},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := clampTTL(c.in); got != c.want {
				t.Fatalf("clampTTL(%v) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}
