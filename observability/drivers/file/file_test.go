package file

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/godx-jp/godx-platform-framework/observability/driver"
)

// assertNotGroupOtherReadable fails if the file at path grants any permission
// bits beyond owner rw + group r (i.e. 0640). Logs may contain request
// metadata or tokens, so they must not be world-readable.
func assertNotGroupOtherReadable(t *testing.T, path string) {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	perm := fi.Mode().Perm()
	if perm&^0o640 != 0 {
		t.Fatalf("log file %s has perm %o; want no bits beyond 0640", path, perm)
	}
}

func TestLogFilePermsNoRotation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX file modes not meaningful on Windows")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")

	d, err := New(context.Background(), driver.Spec{
		Name:            Name,
		LogFilePath:     path,
		LogFileRotation: RotationNone,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = d.Shutdown(context.Background()) })

	assertNotGroupOtherReadable(t, path)
}

func TestLogFilePermsRotating(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX file modes not meaningful on Windows")
	}
	for _, rotation := range []string{RotationSize, RotationDaily} {
		t.Run(rotation, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "app.log")

			d, err := New(context.Background(), driver.Spec{
				Name:            Name,
				LogFilePath:     path,
				LogFileRotation: rotation,
			})
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			t.Cleanup(func() { _ = d.Shutdown(context.Background()) })

			assertNotGroupOtherReadable(t, path)
		})
	}
}

func TestLogDirPerms(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX file modes not meaningful on Windows")
	}
	dir := t.TempDir()
	logDir := filepath.Join(dir, "logs")
	path := filepath.Join(logDir, "app.log")

	d, err := New(context.Background(), driver.Spec{
		Name:            Name,
		LogFilePath:     path,
		LogFileRotation: RotationNone,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = d.Shutdown(context.Background()) })

	fi, err := os.Stat(logDir)
	if err != nil {
		t.Fatalf("stat %s: %v", logDir, err)
	}
	if perm := fi.Mode().Perm(); perm&^0o750 != 0 {
		t.Fatalf("log dir %s has perm %o; want no bits beyond 0750", logDir, perm)
	}
}

func TestRegisteredOnImport(t *testing.T) {
	if _, ok := driver.Lookup(Name); !ok {
		t.Fatal("file driver not registered")
	}
}
