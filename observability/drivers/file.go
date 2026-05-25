package drivers

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

// fileDriver writes structured JSON logs to a local file, with optional
// rotation. Like the `stdout` driver, traces are kept in-process (so
// `trace_id` still appears in log records) and metrics are no-op — the
// driver is a log channel, not a transport.
//
// Suitable for: bare-metal / VM deployments, projects that cannot afford a
// hosted backend, dev environments where stdout is too noisy. Not
// recommended for containerised workloads (use `stdout` so the orchestrator
// can collect logs instead).
type fileDriver struct {
	handler slog.Handler
	tp      *sdktrace.TracerProvider
	close   func() error

	stopRot  chan struct{}
	stopOnce sync.Once
}

// File rotation modes accepted via OBSERVABILITY_LOG_FILE_ROTATION.
const (
	LogFileRotationNone  = "none"  // append-only, no rotation
	LogFileRotationDaily = "daily" // rotate at local midnight
	LogFileRotationSize  = "size"  // rotate when MaxSize is reached
)

func newFile(s Spec) (*fileDriver, error) {
	if s.LogFilePath == "" {
		return nil, fmt.Errorf("file driver: LogFilePath required (set OBSERVABILITY_LOG_FILE_PATH)")
	}

	abs, err := filepath.Abs(s.LogFilePath)
	if err != nil {
		return nil, fmt.Errorf("file driver: resolve path %q: %w", s.LogFilePath, err)
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return nil, fmt.Errorf("file driver: create log dir: %w", err)
	}

	rotation := strings.ToLower(s.LogFileRotation)
	if rotation == "" {
		rotation = LogFileRotationDaily
	}

	var (
		w       io.WriteCloser
		lj      *lumberjack.Logger
		closeFn func() error
		isDaily bool
	)
	switch rotation {
	case LogFileRotationNone:
		f, err := os.OpenFile(abs, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, fmt.Errorf("file driver: open %s: %w", abs, err)
		}
		w = f
		closeFn = f.Close

	case LogFileRotationSize, LogFileRotationDaily:
		lj = &lumberjack.Logger{
			Filename:   abs,
			MaxSize:    nonZeroInt(s.LogFileMaxSizeMB, 100),
			MaxAge:     s.LogFileMaxAgeDays,
			MaxBackups: s.LogFileMaxBackups,
			Compress:   s.LogFileCompress,
			LocalTime:  true,
		}
		w = lj
		closeFn = lj.Close
		isDaily = rotation == LogFileRotationDaily

	default:
		return nil, fmt.Errorf("file driver: unknown OBSERVABILITY_LOG_FILE_ROTATION %q (valid: none, size, daily)", rotation)
	}

	d := &fileDriver{
		handler: slog.NewJSONHandler(w, &slog.HandlerOptions{Level: s.LogLevel}),
		tp: sdktrace.NewTracerProvider(
			sdktrace.WithSampler(samplerFor(s.TraceSampleRate)),
			sdktrace.WithResource(resourceFor(s)),
		),
		close: closeFn,
	}

	if isDaily {
		d.startDailyRotator(lj)
	}
	return d, nil
}

func (d *fileDriver) LoggerHandler() slog.Handler          { return d.handler }
func (d *fileDriver) TracerProvider() trace.TracerProvider { return d.tp }
func (d *fileDriver) MeterProvider() metric.MeterProvider  { return metricnoop.NewMeterProvider() }

func (d *fileDriver) Shutdown(ctx context.Context) error {
	d.stopOnce.Do(func() {
		if d.stopRot != nil {
			close(d.stopRot)
		}
	})
	if err := d.tp.Shutdown(ctx); err != nil {
		return err
	}
	if d.close != nil {
		return d.close()
	}
	return nil
}

// startDailyRotator spawns a goroutine that calls lumberjack.Rotate() at
// local midnight. Lumberjack's native rotation is size-based; this wrapper
// adds time-based rotation on top.
func (d *fileDriver) startDailyRotator(lj *lumberjack.Logger) {
	d.stopRot = make(chan struct{})
	stop := d.stopRot
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
