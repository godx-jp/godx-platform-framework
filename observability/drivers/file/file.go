// Package file is the local-file observability driver — Laravel's `single`
// and `daily` log channels for Go. Writes structured JSON to a path with
// optional rotation by size or local date; traces are kept in-process so
// trace_id still appears in records. Auto-registered when the observability
// package is imported.
//
// Suitable for bare-metal / VM deployments and projects that cannot afford
// a hosted backend. Not recommended for containerised workloads — use the
// stdout driver and let the orchestrator collect logs instead.
package file

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

	"github.com/godx-jp/godx-platform-framework/observability/driver"
)

// Name is the identifier used by OBSERVABILITY_DRIVER to select this driver.
const Name = "file"

// Rotation modes accepted via OBSERVABILITY_LOG_FILE_ROTATION.
const (
	RotationNone  = "none"  // append-only, no rotation
	RotationDaily = "daily" // rotate at local midnight (default)
	RotationSize  = "size"  // rotate when MaxSize is reached
)

func init() { driver.Register(Name, New) }

// New constructs the file driver from spec. Returns an error if LogFilePath
// is empty, the parent directory cannot be created, the file cannot be
// opened, or LogFileRotation is unrecognised.
func New(_ context.Context, s driver.Spec) (driver.Driver, error) {
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
		rotation = RotationDaily
	}

	var (
		w       io.WriteCloser
		lj      *lumberjack.Logger
		closeFn func() error
		isDaily bool
	)
	switch rotation {
	case RotationNone:
		f, err := os.OpenFile(abs, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, fmt.Errorf("file driver: open %s: %w", abs, err)
		}
		w = f
		closeFn = f.Close

	case RotationSize, RotationDaily:
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
		isDaily = rotation == RotationDaily

	default:
		return nil, fmt.Errorf("file driver: unknown OBSERVABILITY_LOG_FILE_ROTATION %q (valid: none, size, daily)", rotation)
	}

	d := &impl{
		handler: slog.NewJSONHandler(w, &slog.HandlerOptions{Level: s.LogLevel}),
		tp: sdktrace.NewTracerProvider(
			sdktrace.WithSampler(driver.SamplerFor(s.TraceSampleRate)),
			sdktrace.WithResource(driver.ResourceFor(s)),
		),
		close: closeFn,
	}
	if isDaily {
		d.startDailyRotator(lj)
	}
	return d, nil
}

type impl struct {
	handler  slog.Handler
	tp       *sdktrace.TracerProvider
	close    func() error
	stopRot  chan struct{}
	stopOnce sync.Once
}

func (d *impl) LoggerHandler() slog.Handler          { return d.handler }
func (d *impl) TracerProvider() trace.TracerProvider { return d.tp }
func (d *impl) MeterProvider() metric.MeterProvider  { return metricnoop.NewMeterProvider() }

func (d *impl) Shutdown(ctx context.Context) error {
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
func (d *impl) startDailyRotator(lj *lumberjack.Logger) {
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
