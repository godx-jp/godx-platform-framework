package observability

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"golang.org/x/time/rate"
)

// ErrorReport is a single error event reported to an [ErrorReporter]. It carries
// the Go error value, a coarse Source classification, an optional severity, and
// arbitrary structured context.
type ErrorReport struct {
	// Err is the error that occurred. Reports with a nil Err are still logged
	// and counted (callers that want to skip nil should do so themselves; the
	// adapter helpers below already skip nil for you).
	Err error
	// Source is a low-cardinality origin label, e.g. "http", "scheduler",
	// "queue", "events", "outbox", "circuitbreaker".
	Source string
	// Severity is the slog level to log at. The zero value (slog.LevelInfo's
	// numeric sibling 0 == slog.LevelInfo) is treated as slog.LevelError so a
	// bare ErrorReport{Err: e} logs as an error.
	Severity slog.Level
	// Attrs is structured context attached to the log record (job name,
	// subject, route, ...). It is not added to the metric to keep cardinality
	// bounded.
	Attrs []slog.Attr
}

// ErrorReporter is the central sink that fans an [ErrorReport] out to the
// structured log, a metric counter, and (optionally) a rate-limited notifier.
//
// Report MUST NOT block the caller: it runs on hot paths (HTTP handlers via
// httpx.SetErrorObserver, background workers) where a slow sink would cause
// head-of-line blocking. The default implementation logs+counts inline (cheap,
// in-memory / no-op meter) and dispatches notifications — which may do network
// I/O — on a background worker with a bounded queue.
type ErrorReporter interface {
	Report(ctx context.Context, r ErrorReport)
}

// Notifier is the minimal alerting sink the reporter dispatches to. The
// notifications module can be adapted to this interface with a small shim so
// the observability package never imports notifications. A nil Notifier on
// [ReporterOptions] disables alerting entirely.
//
// Notify is invoked from the reporter's own background goroutine with a bounded
// context (see [ReporterOptions.NotifyTimeout]) — never on the caller's
// goroutine — so a slow notifier cannot block request handling.
type Notifier interface {
	Notify(ctx context.Context, subject, body string) error
}

// ReporterOptions configures the optional alerting behaviour of a reporter.
type ReporterOptions struct {
	// Notifier, when non-nil, receives rate-limited alerts for reports whose
	// severity is >= NotifyMinLevel. Nil disables alerting (and the worker).
	Notifier Notifier
	// NotifyMinLevel is the minimum severity that triggers a notification. The
	// zero value is treated as slog.LevelError.
	NotifyMinLevel slog.Level
	// NotifyRate is the sustained notification rate per (source+error.type)
	// key. The zero value defaults to defaultNotifyRate (one per minute).
	NotifyRate rate.Limit
	// NotifyBurst is the token-bucket burst size per key. The zero value
	// defaults to defaultNotifyBurst (1) — at most one alert before refill.
	NotifyBurst int
	// NotifyBuffer is the size of the background notification queue. When the
	// queue is full, further notifications are dropped (counted via
	// [Reporter.NotificationsDropped]) rather than blocking the caller. The zero
	// value defaults to defaultNotifyBuffer.
	NotifyBuffer int
	// NotifyTimeout bounds each Notifier.Notify call so a hung endpoint cannot
	// pin the worker. The zero value defaults to defaultNotifyTimeout.
	NotifyTimeout time.Duration
}

const (
	// defaultNotifyRate allows one notification per minute, per
	// (source+error.type) key, after the initial burst. rate.Limit is ev/sec.
	defaultNotifyRate    = rate.Limit(1.0 / 60.0)
	defaultNotifyBurst   = 1
	defaultNotifyBuffer  = 256
	defaultNotifyTimeout = 5 * time.Second
)

// NewReporter builds the default Provider-backed reporter. The returned reporter
// is safe for concurrent use and never blocks the caller. A nil Provider falls
// back to the package global / stdout provider so the reporter is always usable.
//
// When a Notifier is configured a background goroutine is started; call
// [Reporter.Shutdown] (e.g. via framework.App.OnShutdown) to drain and stop it.
func NewReporter(p *Provider, opts ReporterOptions) *Reporter {
	if p == nil {
		p = globalProvider()
	}

	minLevel := opts.NotifyMinLevel
	if minLevel == 0 {
		minLevel = slog.LevelError
	}
	r := opts.NotifyRate
	if r == 0 {
		r = defaultNotifyRate
	}
	burst := opts.NotifyBurst
	if burst <= 0 {
		burst = defaultNotifyBurst
	}
	buffer := opts.NotifyBuffer
	if buffer <= 0 {
		buffer = defaultNotifyBuffer
	}
	timeout := opts.NotifyTimeout
	if timeout <= 0 {
		timeout = defaultNotifyTimeout
	}

	// errors.reported: a convenience counter for non-HTTP error sources. HTTP
	// error rate still comes from the Phase-1 duration histogram. When the
	// active driver has a no-op meter (stdout/file/cloudwatch) the counter is a
	// cheap no-op. A registration error leaves counter nil; Report handles that.
	counter, err := p.Meter().Int64Counter(
		"errors.reported",
		metric.WithUnit("{error}"),
		metric.WithDescription("Errors reported through the central ErrorReporter, keyed by source and error.type."),
	)
	if err != nil {
		p.Logger().Warn("observability: errors.reported counter registration failed", "error", err)
		counter = nil
	}

	rp := &Reporter{
		provider:      p,
		counter:       counter,
		notifier:      opts.Notifier,
		notifyLevel:   minLevel,
		notifyTimeout: timeout,
		rate:          r,
		burst:         burst,
		limiters:      make(map[string]*rate.Limiter),
	}
	if rp.notifier != nil {
		rp.notifyCh = make(chan notifyJob, buffer)
		rp.wg.Add(1)
		go rp.notifyWorker()
	}
	return rp
}

type notifyJob struct {
	subject string
	body    string
}

// Reporter is the default [ErrorReporter]. Construct it with [NewReporter].
type Reporter struct {
	provider      *Provider
	counter       metric.Int64Counter
	notifier      Notifier
	notifyLevel   slog.Level
	notifyTimeout time.Duration
	rate          rate.Limit
	burst         int

	mu       sync.Mutex
	limiters map[string]*rate.Limiter

	notifyCh  chan notifyJob
	wg        sync.WaitGroup
	closeOnce sync.Once
	dropped   atomic.Int64
}

// Report logs the error at the resolved severity, increments errors.reported,
// and (when a notifier is configured and the severity meets the threshold)
// enqueues a rate-limited notification onto the background worker. It never
// blocks on the notifier, panics, or returns errors. Safe for concurrent use.
func (rp *Reporter) Report(ctx context.Context, r ErrorReport) {
	level := r.Severity
	if level == 0 {
		level = slog.LevelError
	}

	errType := errorTypeName(r.Err)
	msg := "error reported"
	if r.Err != nil {
		msg = r.Err.Error()
	}

	// 1. Log (context-aware so trace_id / correlation_id are attached). When the
	// provider's async-logging is enabled this is itself non-blocking.
	attrs := make([]slog.Attr, 0, len(r.Attrs)+2)
	attrs = append(attrs,
		slog.String("source", r.Source),
		slog.String("error.type", errType),
	)
	attrs = append(attrs, r.Attrs...)
	rp.provider.Logger().LogAttrs(ctx, level, msg, attrs...)

	// 2. Metric: increment errors.reported keyed by source + error.type.
	if rp.counter != nil {
		rp.counter.Add(ctx, 1, metric.WithAttributes(
			attribute.String("source", r.Source),
			attribute.String("error.type", errType),
		))
	}

	// 3. Notify (optional, rate-limited, NON-BLOCKING). Network I/O happens on
	// the background worker, never on the caller's goroutine.
	if rp.notifier == nil || level < rp.notifyLevel {
		return
	}
	key := r.Source + "\x00" + errType
	if !rp.allow(key) {
		return // dropped by the limiter
	}
	job := notifyJob{
		subject: fmt.Sprintf("[%s] %s", r.Source, errType),
		body:    msg,
	}
	select {
	case rp.notifyCh <- job:
	default:
		// Queue full: drop rather than block the caller. Visible via
		// NotificationsDropped(); the alert backlog, not the app, absorbs spikes.
		rp.dropped.Add(1)
	}
}

// notifyWorker drains the notification queue and dispatches each alert with a
// bounded, background context (not the caller's request context, which is
// cancelled once the request completes).
func (rp *Reporter) notifyWorker() {
	defer rp.wg.Done()
	for job := range rp.notifyCh {
		ctx, cancel := context.WithTimeout(context.Background(), rp.notifyTimeout)
		if err := rp.notifier.Notify(ctx, job.subject, job.body); err != nil {
			rp.provider.Logger().Warn("observability: error notification failed", "error", err)
		}
		cancel()
	}
}

// Shutdown stops the notification worker, draining queued alerts until ctx is
// done. Safe to call multiple times; a no-op when no notifier is configured.
func (rp *Reporter) Shutdown(ctx context.Context) error {
	if rp.notifyCh == nil {
		return nil
	}
	rp.closeOnce.Do(func() { close(rp.notifyCh) })
	done := make(chan struct{})
	go func() { rp.wg.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// NotificationsDropped reports how many notifications were dropped because the
// background queue was full. A persistently growing value means the alert rate
// exceeds NotifyBuffer drainage — widen the buffer or tighten NotifyRate.
func (rp *Reporter) NotificationsDropped() int64 { return rp.dropped.Load() }

// allow reports whether a notification for key may be sent now, consuming one
// token from that key's bucket. The limiter map is guarded by a mutex so Report
// is safe under concurrency.
func (rp *Reporter) allow(key string) bool {
	rp.mu.Lock()
	lim, ok := rp.limiters[key]
	if !ok {
		lim = rate.NewLimiter(rp.rate, rp.burst)
		rp.limiters[key] = lim
	}
	rp.mu.Unlock()
	return lim.Allow()
}

// errorTypeName returns the Go type name of err (e.g. "*fmt.wrapError"), or
// "<nil>" when err is nil. It mirrors the error.type attribute used by the
// HTTP metrics so the two error surfaces share a label vocabulary.
func errorTypeName(err error) string {
	if err == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%T", err)
}

// --- Adapters -------------------------------------------------------------
//
// These return closures whose signatures match the existing per-module hooks so
// callers can wire them without observability importing httpx / events /
// scheduler / messaging-outbox. They compile against plain func types.

// HTTPErrorObserver adapts an ErrorReporter to the httpx.ErrorObserver hook
// (func(ctx, err, status)). Severity is derived from the status code:
// >=500 => Error, >=400 => Warn, else Info. A nil err is skipped (no report).
//
//	httpx.SetErrorObserver(observability.HTTPErrorObserver(rep))
func HTTPErrorObserver(rep ErrorReporter) func(ctx context.Context, err error, status int) {
	return func(ctx context.Context, err error, status int) {
		if err == nil {
			return
		}
		var level slog.Level
		switch {
		case status >= 500:
			level = slog.LevelError
		case status >= 400:
			level = slog.LevelWarn
		default:
			level = slog.LevelInfo
		}
		rep.Report(ctx, ErrorReport{
			Err:      err,
			Source:   "http",
			Severity: level,
			Attrs:    []slog.Attr{slog.Int("http.response.status_code", status)},
		})
	}
}

// ErrorHook adapts an ErrorReporter to the func(error) hooks used by the events
// async bus (Options.OnError) and the outbox poller (Options.OnError). A nil
// error is skipped. The report uses the given source and the default (Error)
// severity.
//
//	events.NewAsync(bus, events.AsyncOptions{OnError: observability.ErrorHook(rep, "events")})
func ErrorHook(rep ErrorReporter, source string) func(error) {
	return func(err error) {
		if err == nil {
			return
		}
		rep.Report(context.Background(), ErrorReport{Err: err, Source: source})
	}
}

// JobErrorHook adapts an ErrorReporter to the scheduler's OnRun hook
// (func(job string, err error)). It is a no-op when err is nil, so it can be
// wired unconditionally; the job name is attached as a structured attribute.
//
//	scheduler.Options{OnRun: observability.JobErrorHook(rep, "scheduler")}
func JobErrorHook(rep ErrorReporter, source string) func(job string, err error) {
	return func(job string, err error) {
		if err == nil {
			return
		}
		rep.Report(context.Background(), ErrorReport{
			Err:    err,
			Source: source,
			Attrs:  []slog.Attr{slog.String("job", job)},
		})
	}
}
