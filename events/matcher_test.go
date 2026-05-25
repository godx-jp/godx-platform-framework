package events

import "testing"

func TestMatch(t *testing.T) {
	cases := []struct {
		pattern string
		event   string
		want    bool
	}{
		{"user.created", "user.created", true},
		{"user.created", "user.deleted", false},
		{"*", "anything", true},
		{"*", "anything.nested.deep", true},
		{"user.*", "user.created", true},
		{"user.*", "user.deleted", true},
		{"user.*", "order.created", false},
		{"user.*", "user.profile.updated", true}, // multi-segment trailing-*
		{"*.deleted", "user.deleted", true},
		{"*.deleted", "order.profile.deleted", true},
		{"*.deleted", "user.created", false},
		{"user.*.email", "user.created.email", true},
		{"user.*.email", "user.deleted.email", true},
		{"user.*.email", "user.deleted.sms", false},
	}
	for _, c := range cases {
		if got := match(c.pattern, c.event); got != c.want {
			t.Errorf("match(%q, %q) = %v, want %v", c.pattern, c.event, got, c.want)
		}
	}
}
