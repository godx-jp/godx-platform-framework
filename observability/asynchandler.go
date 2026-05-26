package observability

// asynchandler.go implements an OPT-IN asynchronous, non-blocking logging
// path for the framework's slog pipeline.
//
// # Why this exists
//
// By default slog handlers write synchronously: the calling goroutine does
// not return from Logger.Info(...) until the underlying sink has accepted the
// record. That is fine for fast sinks, but it turns a stalled sink into a
// liveness hazard for the whole process: a full disk, a stdout pipe whose
// reader is stuck, or a slow network log handler will block every goroutine
// that logs — including HTTP request handlers — producing head-of-line
// blocking and cascading failure.
//
// [NonBlockingHandler] follows the industry-standard mitigation (Logback
// AsyncAppender with neverBlock, Log4j2 AsyncLogger, Zap's
// BufferedWriteSyncer): a bounded in-memory queue drained by a background
// worker. When the queue is full the handler DROPS the record and increments
// a counter rather than blocking the caller. This trades guaranteed delivery
// for guaranteed non-blocking behavior — under a sustained sink stall some
// log records are lost, but the application keeps serving traffic. Lost
// records are counted and surfaced via [NonBlockingHandler.Dropped] so the
// loss is observable rather than silent.
//
// Traces and metrics are unaffected: the OpenTelemetry SDK already exports
// them asynchronously via batching processors, so only the synchronous slog
// log path needs this treatment.

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
)

// defaultAsyncBufferSize is used when NewNonBlockingHandler is given a
// non-positive bufferSize. It is large enough to absorb normal bursts while
// bounding worst-case memory if the sink stalls.
const defaultAsyncBufferSize = 4096

// asyncEntry is one queued log record together with the SPECIFIC inner
// handler that must process it. The handler is carried per-entry because
// WithAttrs / WithGroup produce derived inner handlers with different bound
// state; the shared worker must invoke the exact handler the record was
// logged through, not some arbitrary "current" handler.
type asyncEntry struct {
	handler slog.Handler
	record  slog.Record
}

// asyncCore is the state shared by a NonBlockingHandler and every handler
// derived from it via WithAttrs / WithGroup. There is exactly one core,
// one channel, one worker goroutine, and one dropped-counter per logical
// async handler tree.
type asyncCore struct {
	ch       chan asyncEntry
	dropped  atomic.Int64
	done     chan struct{} // closed by the worker when it has fully drained
	closeOne sync.Once     // guards closing ch exactly once across the tree
}

// NonBlockingHandler is a slog.Handler that wraps an inner handler so that
// Handle NEVER blocks the calling goroutine. Records are placed on a bounded
// channel and processed by a single background worker; if the channel is full
// the record is dropped (and counted) instead of blocking.
//
// Use it only when non-blocking behavior matters more than guaranteed
// delivery — e.g. logging on the hot path of HTTP request handlers, where a
// stalled log sink must not be allowed to stall request processing. For
// audit-grade logging that must not lose records, do NOT wrap with this
// handler.
//
// A NonBlockingHandler and all handlers derived from it via WithAttrs /
// WithGroup share a single channel, worker goroutine, dropped counter, and
// shutdown state. Calling [NonBlockingHandler.Shutdown] on any of them shuts
// down the whole tree.
type NonBlockingHandler struct {
	inner slog.Handler
	core  *asyncCore
}

// NewNonBlockingHandler wraps inner so its Handle calls run on a background
// worker and never block the caller. bufferSize bounds the in-flight queue;
// a value <= 0 selects a sensible default. The worker goroutine is started
// immediately and runs until [NonBlockingHandler.Shutdown] is called.
func NewNonBlockingHandler(inner slog.Handler, bufferSize int) *NonBlockingHandler {
	if bufferSize <= 0 {
		bufferSize = defaultAsyncBufferSize
	}
	core := &asyncCore{
		ch:   make(chan asyncEntry, bufferSize),
		done: make(chan struct{}),
	}
	h := &NonBlockingHandler{inner: inner, core: core}
	go core.run()
	return h
}

// run is the single background worker. It drains the channel until it is
// closed, invoking each entry's handler with a fresh background context.
func (c *asyncCore) run() {
	defer close(c.done)
	for e := range c.ch {
		// Use a background context: the caller's context may already be
		// cancelled by the time the worker runs this entry. The framework's
		// contextHandler has already enriched the record with trace_id /
		// correlation_id before it reached this handler, so the original
		// context is no longer needed for correctness.
		_ = e.handler.Handle(context.Background(), e.record)
	}
}

// Enabled delegates to the inner handler.
func (h *NonBlockingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

// Handle enqueues the record for asynchronous processing and returns
// immediately. It never blocks and never returns an error: if the queue is
// full the record is dropped and the dropped counter is incremented.
//
// The record is cloned before queuing because slog reuses the backing
// storage of a Record after Handle returns.
func (h *NonBlockingHandler) Handle(_ context.Context, r slog.Record) error {
	e := asyncEntry{handler: h.inner, record: r.Clone()}
	select {
	case h.core.ch <- e:
	default:
		h.core.dropped.Add(1)
	}
	return nil
}

// WithAttrs returns a derived handler whose inner handler carries attrs. The
// derived handler SHARES the channel, worker, dropped counter, and shutdown
// state of the receiver — only the inner handler differs.
func (h *NonBlockingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	return &NonBlockingHandler{inner: h.inner.WithAttrs(attrs), core: h.core}
}

// WithGroup returns a derived handler whose inner handler opens a group. The
// derived handler SHARES the channel, worker, dropped counter, and shutdown
// state of the receiver — only the inner handler differs.
func (h *NonBlockingHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	return &NonBlockingHandler{inner: h.inner.WithGroup(name), core: h.core}
}

// Dropped reports how many records have been dropped due to a full queue
// since this handler tree was created. It is safe to call concurrently.
func (h *NonBlockingHandler) Dropped() int64 {
	return h.core.dropped.Load()
}

// Shutdown stops accepting new records (closing the shared channel exactly
// once) and waits for the worker to drain everything already queued, bounded
// by ctx. If ctx is cancelled or its deadline elapses before the worker
// finishes, Shutdown returns the context error; the worker keeps draining in
// the background. Shutdown is idempotent and safe to call from any handler in
// the tree.
func (h *NonBlockingHandler) Shutdown(ctx context.Context) error {
	h.core.closeOne.Do(func() { close(h.core.ch) })
	select {
	case <-h.core.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
