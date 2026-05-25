package backends

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"gopkg.in/natefinch/lumberjack.v2"
)

// fileBackend writes structured JSON logs to a local file, with optional
// rotation. Like the `stdout` driver, traces are kept in-process (so
// `trace_id` still appears in log records) and metrics are no-op — the
// driver is a log channel, not a transport.
//
// Suitable for: bare-metal / VM deployments, projects that cannot afford a
// hosted backend, dev environments where stdout is too noisy. Not
// recommended for containerised workloads (use `stdout` so the orchestrator
// can collect logs instead).
type fileBackend struct {
	handler slog.Handler
	tp      *sdktrace.TracerProvider
	close   func() error

	stopRot  chan struct{} // nil when rotation goroutine not running
	stopOnce sync.Once
}

// File rotation modes accepted via OBS_LOG_ROTATE.
const (
	FileRotateNone  = "none"  // append-only, no rotation
	FileRotateDaily = "daily" // rotate at local midnight
	FileRotateSize  = "size"  // rotate when MaxSize is reached
)

func newFile(s Spec) (*fileBackend, error) {
	if s.FilePath == "" {
		return nil, fmt.Errorf("file backend: FilePath required (set OBS_LOG_FILE)")
	}

	abs, err := filepath.Abs(s.FilePath)
	if err != nil {
		return nil, fmt.Errorf("file backend: resolve path %q: %w", s.FilePath, err)
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return nil, fmt.Errorf("file backend: create log dir: %w", err)
	}

	rotate := strings.ToLower(s.FileRotate)
	if rotate == "" {
		rotate = FileRotateDaily
	}

	var (
		w        io.WriteCloser
		lj       *lumberjack.Logger
		closeFn  func() error
		isDaily  bool
	)
	switch rotate {
	case FileRotateNone:
		f, err := os.OpenFile(abs, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, fmt.Errorf("file backend: open %s: %w", abs, err)
		}
		w = f
		closeFn = f.Close

	case FileRotateSize, FileRotateDaily:
		lj = &lumberjack.Logger{
			Filename:   abs,
			MaxSize:    nonZeroInt(s.FileMaxSizeMB, 100),
			MaxAge:     s.FileMaxAgeDays,
			MaxBackups: s.FileMaxBackups,
			Compress:   s.FileCompress,
			LocalTime:  true,
		}
		w = lj
		closeFn = lj.Close
		isDaily = rotate == FileRotateDaily

	default:
		return nil, fmt.Errorf("file backend: unknown OBS_LOG_ROTATE %q (valid: none, size, daily)", rotate)
	}

	be := &fileBackend{
		handler: slog.NewJSONHandler(w, &slog.HandlerOptions{Level: s.LogLevel}),
		tp: sdktrace.NewTracerProvider(
			sdktrace.WithSampler(samplerFor(s.TraceSampleRate)),
			sdktrace.WithResource(resourceFor(s)),
		),
		close: closeFn,
	}

	if isDaily {
		be.startDailyRotator(lj)
	}
	return be, nil
}

func (b *fileBackend) LoggerHandler() slog.Handler          { return b.handler }
func (b *fileBackend) TracerProvider() trace.TracerProvider { return b.tp }
func (b *fileBackend) MeterProvider() metric.MeterProvider  { return metricnoop.NewMeterProvider() }

func (b *fileBackend) Shutdown(ctx context.Context) error {
	b.stopOnce.Do(func() {
		if b.stopRot != nil {
			close(b.stopRot)
		}
	})
	if err := b.tp.Shutdown(ctx); err != nil {
		return err
	}
	if b.close != nil {
		return b.close()
	}
	return nil
}

// startDailyRotator spawns a goroutine that calls lumberjack.Rotate() at
// local midnight. Lumberjack's native rotation is size-based; this wrapper
// adds time-based rotation on top.
func (b *fileBackend) startDailyRotator(lj *lumberjack.Logger) {
	b.stopRot = make(chan struct{})
	stop := b.stopRot
	go func() {
		for {
			wait := time.Until(nextMidnight(time.Now()))
			t := time.NewTimer(wait)
			select {
			case <-t.C:
				_ = lj.Rotate()
			case <-stop:
				t.Stop()
				return
			}
		}
	}()
}

func nextMidnight(now time.Time) time.Time {
	y, m, d := now.Date()
	return time.Date(y, m, d+1, 0, 0, 0, 0, now.Location())
}

func nonZeroInt(v, def int) int {
	if v > 0 {
		return v
	}
	return def
}
