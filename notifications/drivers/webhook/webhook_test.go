package webhook

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/godx-jp/godx-platform-framework/notifications/contract"
	"github.com/godx-jp/godx-platform-framework/notifications/internal/urlguard"
)

// spyDoer records whether an HTTP request was ever issued.
type spyDoer struct{ called bool }

func (s *spyDoer) Do(*http.Request) (*http.Response, error) {
	s.called = true
	return &http.Response{StatusCode: 200, Body: http.NoBody}, nil
}

type notifiable struct{ route string }

func (n notifiable) RouteNotificationFor(string) string { return n.route }

type presenter struct{ msg contract.WebhookMessage }

func (p presenter) ToWebhook(contract.Notifiable) contract.WebhookMessage { return p.msg }

func TestWebhook_BlockedURL_NoHTTPCall(t *testing.T) {
	spy := &spyDoer{}
	ch := New("", spy)

	err := ch.Send(context.Background(), notifiable{}, presenter{
		msg: contract.WebhookMessage{URL: "http://169.254.169.254/latest/meta-data/"},
	})
	if err == nil {
		t.Fatalf("expected error for blocked URL, got nil")
	}
	if !errorsContain(err, urlguard.ErrBlockedURL.Error()) {
		t.Fatalf("expected SSRF guard error, got %v", err)
	}
	if spy.called {
		t.Fatalf("HTTP request was issued to a blocked URL; SSRF guard bypassed")
	}
}

func TestWebhook_RouteBlocked_NoHTTPCall(t *testing.T) {
	spy := &spyDoer{}
	ch := New("", spy)

	// URL comes from the notifiable's route (attacker-influenced path).
	err := ch.Send(context.Background(), notifiable{route: "http://localhost:6379/"}, presenter{
		msg: contract.WebhookMessage{},
	})
	if err == nil {
		t.Fatalf("expected error for blocked route URL, got nil")
	}
	if spy.called {
		t.Fatalf("HTTP request was issued to a blocked route URL")
	}
}

func errorsContain(err error, sub string) bool {
	return err != nil && strings.Contains(err.Error(), sub)
}
