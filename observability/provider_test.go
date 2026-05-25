package observability_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/godx-jp/godx-platform-framework/framework"
	"github.com/godx-jp/godx-platform-framework/observability"
	// Blank-import the heavy drivers under test so the registry resolves
	// "otlp" and "cloudwatch" — mirrors how a real consumer would opt in.
	_ "github.com/godx-jp/godx-platform-framework/observability/drivers/cloudwatch"
	_ "github.com/godx-jp/godx-platform-framework/observability/drivers/otlp"
)

// captureStdout swaps os.Stdout for a pipe, runs fn, then returns whatever
// the fn produced. The stdout driver writes there directly so we can
// inspect the JSON record format.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	var buf bytes.Buffer
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = buf.ReadFrom(r)
	}()

	fn()

	_ = w.Close()
	wg.Wait()
	os.Stdout = orig
	return buf.String()
}

func TestProvider_Stdout_LoggerEmitsServiceAttrs(t *testing.T) {
	out := captureStdout(t, func() {
		p, err := observability.NewProvider(context.Background(), observability.Config{
			ServiceName:    "svc-A",
			ServiceVersion: "9.9.9",
			Environment:    "test",
			Driver:         observability.DriverStdout,
			LogLevel:       slog.LevelInfo,
		})
		if err != nil {
			t.Fatalf("NewProvider: %v", err)
		}
		defer p.Shutdown(context.Background())

		p.Logger().Info("hello")
	})

	rec := decodeFirstJSON(t, out)
	if rec["service"] != "svc-A" || rec["version"] != "9.9.9" || rec["env"] != "test" || rec["msg"] != "hello" {
		t.Fatalf("missing fields in record: %+v", rec)
	}
}

func TestProvider_LoggerInjectsTraceID(t *testing.T) {
	out := captureStdout(t, func() {
		p, err := observability.NewProvider(context.Background(), observability.Config{
			ServiceName: "svc",
			Driver:      observability.DriverStdout,
		})
		if err != nil {
			t.Fatalf("NewProvider: %v", err)
		}
		defer p.Shutdown(context.Background())

		ctx, span := p.Tracer().Start(context.Background(), "op")
		defer span.End()
		p.Logger().InfoContext(ctx, "in-span")
	})

	rec := decodeFirstJSON(t, out)
	tid, ok := rec["trace_id"].(string)
	if !ok || len(tid) != 32 {
		t.Fatalf("trace_id missing or malformed: %v (record=%+v)", tid, rec)
	}
}

func TestProvider_LoggerInjectsCorrelationID(t *testing.T) {
	out := captureStdout(t, func() {
		p, err := observability.NewProvider(context.Background(), observability.Config{
			ServiceName: "svc",
			Driver:      observability.DriverStdout,
		})
		if err != nil {
			t.Fatalf("NewProvider: %v", err)
		}
		defer p.Shutdown(context.Background())

		ctx := observability.ContextWithCorrelationID(context.Background(), "abc-123")
		p.Logger().InfoContext(ctx, "with-correlation")
	})

	rec := decodeFirstJSON(t, out)
	if got := rec["correlation_id"]; got != "abc-123" {
		t.Fatalf("correlation_id = %v, want abc-123", got)
	}
}

func TestProvider_OTLP_RequiresEndpoint(t *testing.T) {
	_, err := observability.NewProvider(context.Background(), observability.Config{
		ServiceName: "svc",
		Driver:      observability.DriverOTLP,
	})
	if err == nil {
		t.Fatalf("expected error for OTLP with no endpoint")
	}
}

func TestProvider_CloudWatch_NotImplementedInV04(t *testing.T) {
	_, err := observability.NewProvider(context.Background(), observability.Config{
		ServiceName: "svc",
		Driver:      observability.DriverCloudWatch,
	})
	if err == nil {
		t.Fatalf("expected ErrNotImplemented for cloudwatch driver in v0.4.x")
	}
	if !strings.Contains(err.Error(), "cloudwatch") {
		t.Fatalf("err should mention cloudwatch: %v", err)
	}
}

func TestModule_WiredIntoApp(t *testing.T) {
	_ = captureStdout(t, func() {
		t.Setenv("OBSERVABILITY_DRIVER", "stdout")
		app := framework.New("module-svc", "1.0.0").Use(observability.Module)
		if err := app.Init(context.Background()); err != nil {
			t.Fatalf("Init: %v", err)
		}
		p := observability.FromApp(app)
		if p.Driver() != "stdout" {
			t.Fatalf("driver = %q, want stdout", p.Driver())
		}
		if err := app.Shutdown(context.Background()); err != nil {
			t.Fatalf("Shutdown: %v", err)
		}
	})
}

func decodeFirstJSON(t *testing.T, out string) map[string]any {
	t.Helper()
	line := firstLine(out)
	if line == "" {
		t.Fatalf("no log line captured: %q", out)
	}
	var rec map[string]any
	if err := json.Unmarshal([]byte(line), &rec); err != nil {
		t.Fatalf("decode JSON %q: %v", line, err)
	}
	return rec
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
