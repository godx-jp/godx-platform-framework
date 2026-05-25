package drivers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// stackDriver is a meta-driver that fans out log records to multiple
// underlying drivers, mirroring Laravel's `stack` log channel. Traces and
// metrics flow through the first sub-driver only — duplicating a span
// across exporters would create double-counted distributed traces.
//
//	OBSERVABILITY_DRIVER=stack
//	OBSERVABILITY_STACK_DRIVERS=stdout,file
//	OBSERVABILITY_LOG_FILE_PATH=/var/log/app.log
type stackDriver struct {
	subs    []Driver
	handler *stackHandler
}

func newStack(ctx context.Context, s Spec) (*stackDriver, error) {
	if len(s.StackDrivers) == 0 {
		return nil, fmt.Errorf("stack driver: StackDrivers required (set OBSERVABILITY_STACK_DRIVERS as a comma-separated list)")
	}

	subs := make([]Driver, 0, len(s.StackDrivers))
	for _, raw := range s.StackDrivers {
		name := strings.ToLower(strings.TrimSpace(raw))
		if name == "" {
			continue
		}
		if name == "stack" {
			return nil, fmt.Errorf("stack driver: cannot nest %q inside itself", name)
		}

		sub := s
		sub.Name = name
		sub.StackDrivers = nil

		d, err := New(ctx, sub)
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
	return &stackDriver{
		subs:    subs,
		handler: &stackHandler{handlers: handlers},
	}, nil
}

func (s *stackDriver) LoggerHandler() slog.Handler          { return s.handler }
func (s *stackDriver) TracerProvider() trace.TracerProvider { return s.subs[0].TracerProvider() }
func (s *stackDriver) MeterProvider() metric.MeterProvider  { return s.subs[0].MeterProvider() }

func (s *stackDriver) Shutdown(ctx context.Context) error {
	var errs []error
	for i := len(s.subs) - 1; i >= 0; i-- {
		if err := s.subs[i].Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// stackHandler dispatches one slog.Record to all underlying handlers. Errors
// are joined so a failing sink does not prevent others from receiving the
// record.
type stackHandler struct {
	handlers []slog.Handler
}

func (h *stackHandler) Enabled(ctx context.Context, lvl slog.Level) bool {
	for _, sub := range h.handlers {
		if sub.Enabled(ctx, lvl) {
			return true
		}
	}
	return false
}

func (h *stackHandler) Handle(ctx context.Context, r slog.Record) error {
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

func (h *stackHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clones := make([]slog.Handler, len(h.handlers))
	for i, sub := range h.handlers {
		clones[i] = sub.WithAttrs(attrs)
	}
	return &stackHandler{handlers: clones}
}

func (h *stackHandler) WithGroup(name string) slog.Handler {
	clones := make([]slog.Handler, len(h.handlers))
	for i, sub := range h.handlers {
		clones[i] = sub.WithGroup(name)
	}
	return &stackHandler{handlers: clones}
}
