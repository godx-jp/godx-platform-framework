package observability_test

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/godx-jp/godx-platform-framework/httpx"
	"github.com/godx-jp/godx-platform-framework/observability"
)

// TestErrorReporterBridgesHTTPErrors exercises the full RFC-0001 chain end to
// end: httpx.Serve → SetErrorObserver → observability.HTTPErrorObserver →
// ErrorReporter → structured log. It proves the httpx ↔ observability bridge
// works without httpx importing observability.
func TestErrorReporterBridgesHTTPErrors(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "app.log")

	ctx := context.Background()
	p, err := observability.NewProvider(ctx, observability.Config{
		ServiceName:     "itest",
		Driver:          observability.DriverFile,
		LogFilePath:     logPath,
		LogFileRotation: "none",
		LogLevel:        slog.LevelInfo,
	})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}

	rep := observability.NewReporter(p, observability.ReporterOptions{})
	httpx.SetErrorObserver(observability.HTTPErrorObserver(rep))
	t.Cleanup(func() { httpx.SetErrorObserver(nil) })

	h := httpx.Serve(func(_ http.ResponseWriter, _ *http.Request) error {
		return httpx.WrapStatus(http.StatusServiceUnavailable, "upstream down", errors.New("boom"))
	})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}

	// Flush the file driver before reading.
	_ = p.Shutdown(ctx)

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	out := string(data)

	// 503 is a server error → reported at ERROR severity, source "http".
	if !strings.Contains(out, `"level":"ERROR"`) {
		t.Errorf("expected ERROR-level report, got:\n%s", out)
	}
	if !strings.Contains(out, `"source":"http"`) {
		t.Errorf("expected source=http, got:\n%s", out)
	}
}
