package log_test

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	mdriver "github.com/godx-jp/godx-platform-framework/mail/driver"
	maillog "github.com/godx-jp/godx-platform-framework/mail/drivers/log"
)

// captureLogs swaps the default slog logger for one writing to buf for
// the duration of fn, then restores it.
func captureLogs(t *testing.T, fn func()) string {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	fn()
	return buf.String()
}

func TestSendDoesNotLogFullBody(t *testing.T) {
	const marker = "TOP_SECRET_RESET_TOKEN_abc123"
	// Place the marker well beyond the preview window so it must not
	// appear in the default output.
	body := strings.Repeat("x", 200) + marker

	out := captureLogs(t, func() {
		tr := maillog.New("noreply@example.com")
		if err := tr.Send(context.Background(), mdriver.Message{
			To:      []string{"user@example.com"},
			Subject: "Reset your password",
			Body:    body,
		}); err != nil {
			t.Fatalf("Send: %v", err)
		}
	})

	if strings.Contains(out, marker) {
		t.Fatalf("default log output leaked secret body marker:\n%s", out)
	}
	if !strings.Contains(out, "body_bytes=") {
		t.Fatalf("expected body length in output, got:\n%s", out)
	}
	if !strings.Contains(out, "body_preview=") {
		t.Fatalf("expected truncated body preview in output, got:\n%s", out)
	}
	// The full body must never be present verbatim.
	if strings.Contains(out, body) {
		t.Fatalf("default log output contained full body:\n%s", out)
	}
}

func TestNewWithBodyLogsFullBody(t *testing.T) {
	const marker = "explicit-optin-body"
	out := captureLogs(t, func() {
		tr := maillog.NewWithBody("noreply@example.com")
		if err := tr.Send(context.Background(), mdriver.Message{
			To:      []string{"user@example.com"},
			Subject: "Hi",
			Body:    marker,
		}); err != nil {
			t.Fatalf("Send: %v", err)
		}
	})
	if !strings.Contains(out, marker) {
		t.Fatalf("opt-in full-body logging missing body, got:\n%s", out)
	}
}
