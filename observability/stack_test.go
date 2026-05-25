package observability_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/godx-jp/godx-platform-framework/observability"
)

func TestStackDriver_FansOutToStdoutAndFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stack.log")

	stdoutCapture := captureStdout(t, func() {
		p, err := observability.NewProvider(context.Background(), observability.Config{
			ServiceName:  "stack-svc",
			Driver:       observability.DriverStack,
			StackDrivers: []string{"stdout", "file"},
			LogFilePath:  path,
		})
		if err != nil {
			t.Fatalf("NewProvider: %v", err)
		}

		p.Logger().Info("fan-out", "key", "value")

		if err := p.Shutdown(context.Background()); err != nil {
			t.Fatalf("Shutdown: %v", err)
		}
	})

	if !strings.Contains(stdoutCapture, `"msg":"fan-out"`) {
		t.Fatalf("stdout did not receive record: %q", stdoutCapture)
	}

	fileBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(fileBytes), `"msg":"fan-out"`) {
		t.Fatalf("file did not receive record: %q", string(fileBytes))
	}
}

func TestStackDriver_RejectsNestedStack(t *testing.T) {
	_, err := observability.NewProvider(context.Background(), observability.Config{
		ServiceName:  "svc",
		Driver:       observability.DriverStack,
		StackDrivers: []string{"stdout", "stack"},
	})
	if err == nil {
		t.Fatalf("expected error for nested stack")
	}
	if !strings.Contains(err.Error(), "nest") {
		t.Fatalf("err should mention nesting: %v", err)
	}
}

func TestStackDriver_RejectsEmptyList(t *testing.T) {
	_, err := observability.NewProvider(context.Background(), observability.Config{
		ServiceName: "svc",
		Driver:      observability.DriverStack,
	})
	if err == nil {
		t.Fatalf("expected error for missing StackDrivers")
	}
	if !strings.Contains(err.Error(), "OBSERVABILITY_STACK_DRIVERS") {
		t.Fatalf("err should reference OBSERVABILITY_STACK_DRIVERS: %v", err)
	}
}

func TestStackDriver_RejectsUnknownSubDriver(t *testing.T) {
	_, err := observability.NewProvider(context.Background(), observability.Config{
		ServiceName:  "svc",
		Driver:       observability.DriverStack,
		StackDrivers: []string{"stdout", "voodoo"},
	})
	if err == nil {
		t.Fatalf("expected error for unknown sub-driver")
	}
	if !strings.Contains(err.Error(), "voodoo") {
		t.Fatalf("err should name the bad driver: %v", err)
	}
}

func TestStackDriver_ConfigFromEnv(t *testing.T) {
	t.Setenv("OBSERVABILITY_STACK_DRIVERS", "stdout, file ,, ")
	cfg := observability.LoadConfigFromEnv()
	if got, want := cfg.StackDrivers, []string{"stdout", "file"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("StackDrivers = %v, want %v", got, want)
	}
}
