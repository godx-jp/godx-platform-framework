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
	"github.com/godx-jp/godx-platform-framework/observability/backends"
)

func TestFileBackend_Single_WritesJSONLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")

	p, err := observability.NewProvider(context.Background(), observability.Config{
		ServiceName: "file-svc",
		Backend:     observability.BackendFile,
		LogLevel:    slog.LevelInfo,
		FilePath:    path,
		FileRotate:  backends.FileRotateNone,
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

func TestFileBackend_AutoCreatesParentDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "deep", "nested", "app.log")

	p, err := observability.NewProvider(context.Background(), observability.Config{
		ServiceName: "svc",
		Backend:     observability.BackendFile,
		FilePath:    path,
		FileRotate:  backends.FileRotateNone,
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

func TestFileBackend_DefaultRotateIsDaily(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")

	p, err := observability.NewProvider(context.Background(), observability.Config{
		ServiceName: "svc",
		Backend:     observability.BackendFile,
		FilePath:    path,
		// FileRotate intentionally empty → daily
	})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	p.Logger().Info("hello")
	if err := p.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
}

func TestFileBackend_UnknownRotateRejected(t *testing.T) {
	dir := t.TempDir()
	_, err := observability.NewProvider(context.Background(), observability.Config{
		ServiceName: "svc",
		Backend:     observability.BackendFile,
		FilePath:    filepath.Join(dir, "x.log"),
		FileRotate:  "monthly",
	})
	if err == nil {
		t.Fatalf("expected error for unknown rotate mode")
	}
	if !strings.Contains(err.Error(), "OBS_LOG_ROTATE") {
		t.Fatalf("err should mention OBS_LOG_ROTATE: %v", err)
	}
}

func TestFileBackend_MissingPathRejected(t *testing.T) {
	_, err := observability.NewProvider(context.Background(), observability.Config{
		ServiceName: "svc",
		Backend:     observability.BackendFile,
		// FilePath missing
	})
	if err == nil || !errors.Is(err, err) {
		t.Fatalf("expected error for missing path")
	}
	if !strings.Contains(err.Error(), "OBS_LOG_FILE") {
		t.Fatalf("err should mention OBS_LOG_FILE: %v", err)
	}
}

func TestFileBackend_LogLevelFiltered(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")

	p, err := observability.NewProvider(context.Background(), observability.Config{
		ServiceName: "svc",
		Backend:     observability.BackendFile,
		LogLevel:    slog.LevelWarn,
		FilePath:    path,
		FileRotate:  backends.FileRotateNone,
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
