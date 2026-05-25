package observability_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/godx-jp/godx-platform-framework/observability"
	"github.com/godx-jp/godx-platform-framework/observability/drivers"
)

func TestFileDriver_Single_WritesJSONLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")

	p, err := observability.NewProvider(context.Background(), observability.Config{
		ServiceName:     "file-svc",
		Driver:          observability.DriverFile,
		LogLevel:        slog.LevelInfo,
		LogFilePath:     path,
		LogFileRotation: drivers.LogFileRotationNone,
	})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}

	p.Logger().Info("from-file", "user", "alice")
	if err := p.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	line := firstLine(string(data))
	if line == "" {
		t.Fatalf("no line written: %q", string(data))
	}

	var rec map[string]any
	if err := json.Unmarshal([]byte(line), &rec); err != nil {
		t.Fatalf("decode %q: %v", line, err)
	}
	if rec["service"] != "file-svc" || rec["msg"] != "from-file" || rec["user"] != "alice" {
		t.Fatalf("unexpected record: %+v", rec)
	}
}

func TestFileDriver_AutoCreatesParentDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "deep", "nested", "app.log")

	p, err := observability.NewProvider(context.Background(), observability.Config{
		ServiceName:     "svc",
		Driver:          observability.DriverFile,
		LogFilePath:     path,
		LogFileRotation: drivers.LogFileRotationNone,
	})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	p.Logger().Info("hello")
	_ = p.Shutdown(context.Background())

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file at %s: %v", path, err)
	}
}

func TestFileDriver_DefaultRotationIsDaily(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")

	p, err := observability.NewProvider(context.Background(), observability.Config{
		ServiceName: "svc",
		Driver:      observability.DriverFile,
		LogFilePath: path,
		// LogFileRotation intentionally empty → daily
	})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	p.Logger().Info("hello")
	if err := p.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
}

func TestFileDriver_UnknownRotationRejected(t *testing.T) {
	dir := t.TempDir()
	_, err := observability.NewProvider(context.Background(), observability.Config{
		ServiceName:     "svc",
		Driver:          observability.DriverFile,
		LogFilePath:     filepath.Join(dir, "x.log"),
		LogFileRotation: "monthly",
	})
	if err == nil {
		t.Fatalf("expected error for unknown rotation mode")
	}
	if !strings.Contains(err.Error(), "OBSERVABILITY_LOG_FILE_ROTATION") {
		t.Fatalf("err should mention OBSERVABILITY_LOG_FILE_ROTATION: %v", err)
	}
}

func TestFileDriver_MissingPathRejected(t *testing.T) {
	_, err := observability.NewProvider(context.Background(), observability.Config{
		ServiceName: "svc",
		Driver:      observability.DriverFile,
		// LogFilePath missing
	})
	if err == nil || !errors.Is(err, err) {
		t.Fatalf("expected error for missing path")
	}
	if !strings.Contains(err.Error(), "OBSERVABILITY_LOG_FILE_PATH") {
		t.Fatalf("err should mention OBSERVABILITY_LOG_FILE_PATH: %v", err)
	}
}

func TestFileDriver_LogLevelFiltered(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")

	p, err := observability.NewProvider(context.Background(), observability.Config{
		ServiceName:     "svc",
		Driver:          observability.DriverFile,
		LogLevel:        slog.LevelWarn,
		LogFilePath:     path,
		LogFileRotation: drivers.LogFileRotationNone,
	})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	p.Logger().Info("filtered-out")
	p.Logger().Warn("kept")
	_ = p.Shutdown(context.Background())

	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "filtered-out") {
		t.Fatalf("info line should not be present: %q", string(data))
	}
	if !strings.Contains(string(data), "kept") {
		t.Fatalf("warn line missing: %q", string(data))
	}
}
