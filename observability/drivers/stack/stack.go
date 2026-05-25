// Package stack is a meta-driver that fans out every log record to multiple
// underlying drivers — Laravel's `stack` log channel for Go. Traces and
// metrics flow through the first sub-driver only; duplicating a span across
// exporters would produce double-counted distributed traces.
//
// Auto-registered when the observability package is imported. Heavy
// sub-drivers (otlp, cloudwatch) still need their own blank import.
//
//	OBSERVABILITY_DRIVER=stack
//	OBSERVABILITY_STACK_DRIVERS=stdout,file
//	OBSERVABILITY_LOG_FILE_PATH=/var/log/app.log
package stack

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/godx-jp/godx-platform-framework/observability/driver"
)

// Name is the identifier used by OBSERVABILITY_DRIVER to select this driver.
const Name = "stack"

func init() { driver.Register(Name, New) }

// New constructs the stack meta-driver. Each entry in s.StackDrivers must
// name an already-registered driver; nesting "stack" inside itself is
// rejected to keep shutdown ordering well-defined.
func New(ctx context.Context, s driver.Spec) (driver.Driver, error) {
	if len(s.StackDrivers) == 0 {
		return nil, fmt.Errorf("stack driver: StackDrivers required (set OBSERVABILITY_STACK_DRIVERS as a comma-separated list)")
	}

	subs := make([]driver.Driver, 0, len(s.StackDrivers))
	for _, raw := range s.StackDrivers {
		name := strings.ToLower(strings.TrimSpace(raw))
		if name == "" {
			continue
		}
		if name == Name {
			return nil, fmt.Errorf("stack driver: cannot nest %q inside itself", name)
		}

		sub := s
		sub.Name = name
		sub.StackDrivers = nil

		d, err := driver.New(ctx, sub)
		if err != nil {
			return nil, fmt.Errorf("stack driver: sub-driver %q: %w", name, err)
		}
		subs = append(subs, d)
	}

	if len(subs) == 0 {
		return nil, fmt.Errorf("stack driver: no usable sub-drivers after parsing %v", s.StackDrivers)
	}

	handlers := make([]slog.Handler, len(subs))
	for i, d := range subs {
		handlers[i] = d.LoggerHandler()
	}
	return &impl{
		subs:    subs,
		handler: &fanoutHandler{handlers: handlers},
	}, nil
}

type impl struct {
	subs    []driver.Driver
	handler *fanoutHandler
}

func (d *impl) LoggerHandler() slog.Handler          { return d.handler }
func (d *impl) TracerProvider() trace.TracerProvider { return d.subs[0].TracerProvider() }
func (d *impl) MeterProvider() metric.MeterProvider  { return d.subs[0].MeterProvider() }

func (d *impl) Shutdown(ctx context.Context) error {
	var errs []error
	for i := len(d.subs) - 1; i >= 0; i-- {
		if err := d.subs[i].Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// fanoutHandler dispatches one slog.Record to every underlying handler.
// Errors are joined so a failing sink does not silence the others.
type fanoutHandler struct {
	handlers []slog.Handler
}

func (h *fanoutHandler) Enabled(ctx context.Context, lvl slog.Level) bool {
	for _, sub := range h.handlers {
		if sub.Enabled(ctx, lvl) {
			return true
		}
	}
	return false
}

func (h *fanoutHandler) Handle(ctx context.Context, r slog.Record) error {
	var errs []error
	for _, sub := range h.handlers {
		if !sub.Enabled(ctx, r.Level) {
			continue
		}
		if err := sub.Handle(ctx, r.Clone()); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (h *fanoutHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clones := make([]slog.Handler, len(h.handlers))
	for i, sub := range h.handlers {
		clones[i] = sub.WithAttrs(attrs)
	}
	return &fanoutHandler{handlers: clones}
}

func (h *fanoutHandler) WithGroup(name string) slog.Handler {
	clones := make([]slog.Handler, len(h.handlers))
	for i, sub := range h.handlers {
		clones[i] = sub.WithGroup(name)
	}
	return &fanoutHandler{handlers: clones}
}
