package observability

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"
)

// blockingHandler blocks every Handle call until release is closed. It records
// how many records it eventually processed.
type blockingHandler struct {
	release <-chan struct{}
	mu      sync.Mutex
	handled int
}

func (h *blockingHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *blockingHandler) Handle(_ context.Context, _ slog.Record) error {
	<-h.release
	h.mu.Lock()
	h.handled++
	h.mu.Unlock()
	return nil
}

func (h *blockingHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *blockingHandler) WithGroup(string) slog.Handler      { return h }

func (h *blockingHandler) count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.handled
}

type capturedRecord struct {
	msg   string
	attrs map[string]string
}

// sharedCapture is a capture handler whose derived (WithAttrs/WithGroup)
// handlers append into the SAME records slice, so we can observe output that
// flows through derived inner handlers. This mirrors how a real handler keeps
// a shared sink while only the bound attrs differ.
type sharedCapture struct {
	mu      *sync.Mutex
	records *[]capturedRecord
	attrs   []slog.Attr
}

func newSharedCapture() *sharedCapture {
	return &sharedCapture{mu: &sync.Mutex{}, records: &[]capturedRecord{}}
}

func (h *sharedCapture) Enabled(context.Context, slog.Level) bool { return true }

func (h *sharedCapture) Handle(_ context.Context, r slog.Record) error {
	rec := capturedRecord{msg: r.Message, attrs: map[string]string{}}
	for _, a := range h.attrs {
		rec.attrs[a.Key] = a.Value.String()
	}
	r.Attrs(func(a slog.Attr) bool {
		rec.attrs[a.Key] = a.Value.String()
		return true
	})
	h.mu.Lock()
	*h.records = append(*h.records, rec)
	h.mu.Unlock()
	return nil
}

func (h *sharedCapture) WithAttrs(attrs []slog.Attr) slog.Handler {
	merged := make([]slog.Attr, 0, len(h.attrs)+len(attrs))
	merged = append(merged, h.attrs...)
	merged = append(merged, attrs...)
	return &sharedCapture{mu: h.mu, records: h.records, attrs: merged}
}

func (h *sharedCapture) WithGroup(string) slog.Handler {
	return &sharedCapture{mu: h.mu, records: h.records, attrs: h.attrs}
}

func (h *sharedCapture) snapshot() []capturedRecord {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]capturedRecord{}, *h.records...)
}

func newRecord(msg string) slog.Record {
	return slog.NewRecord(time.Now(), slog.LevelInfo, msg, 0)
}

// TestNonBlockingHandler_DropsAndDoesNotBlock verifies that with a blocked
// inner sink, Handle returns promptly and overflow records are dropped (not
// blocked), then releasing the sink lets Shutdown drain.
func TestNonBlockingHandler_DropsAndDoesNotBlock(t *testing.T) {
	release := make(chan struct{})
	inner := &blockingHandler{release: release}
	h := NewNonBlockingHandler(inner, 4)

	const n = 1000
	start := time.Now()
	for i := 0; i < n; i++ {
		if err := h.Handle(context.Background(), newRecord("msg")); err != nil {
			t.Fatalf("Handle returned error: %v", err)
		}
	}
	elapsed := time.Since(start)

	// 1000 synchronous blocked calls would hang forever; this must be fast.
	if elapsed > 2*time.Second {
		t.Fatalf("Handle calls took %v, expected near-instant (non-blocking)", elapsed)
	}
	if h.Dropped() == 0 {
		t.Fatalf("expected Dropped() > 0 with a blocked sink and tiny buffer, got 0")
	}

	// Release the sink and drain.
	close(release)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := h.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown after release: %v", err)
	}
	if inner.count() == 0 {
		t.Fatalf("expected inner handler to process at least some records after release")
	}
}

// TestNonBlockingHandler_HappyPathDelivers verifies that in the unblocked case
// all records reach the inner handler after Shutdown.
func TestNonBlockingHandler_HappyPathDelivers(t *testing.T) {
	cap := newSharedCapture()
	h := NewNonBlockingHandler(cap, 1024)

	const n = 500
	for i := 0; i < n; i++ {
		if err := h.Handle(context.Background(), newRecord("hello")); err != nil {
			t.Fatalf("Handle: %v", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := h.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if got := len(cap.snapshot()); got != n {
		t.Fatalf("observed %d records, want %d (dropped=%d)", got, n, h.Dropped())
	}
	if h.Dropped() != 0 {
		t.Fatalf("unexpected drops on happy path: %d", h.Dropped())
	}
}

// TestNonBlockingHandler_WithAttrsAndGroupShareWorker verifies derived
// handlers deliver through the shared worker and carry their bound attrs.
func TestNonBlockingHandler_WithAttrsAndGroupShareWorker(t *testing.T) {
	cap := newSharedCapture()
	base := NewNonBlockingHandler(cap, 1024)

	derived := base.WithAttrs([]slog.Attr{slog.String("svc", "billing")})
	grouped := derived.WithGroup("req")

	if err := base.Handle(context.Background(), newRecord("base")); err != nil {
		t.Fatalf("base.Handle: %v", err)
	}
	if err := derived.Handle(context.Background(), newRecord("derived")); err != nil {
		t.Fatalf("derived.Handle: %v", err)
	}
	if err := grouped.Handle(context.Background(), newRecord("grouped")); err != nil {
		t.Fatalf("grouped.Handle: %v", err)
	}

	// Derived handlers share the same core: shutting down via base drains all.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := base.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	recs := cap.snapshot()
	if len(recs) != 3 {
		t.Fatalf("observed %d records through shared worker, want 3", len(recs))
	}
	var derivedSeen bool
	for _, r := range recs {
		if r.msg == "derived" || r.msg == "grouped" {
			if r.attrs["svc"] != "billing" {
				t.Fatalf("derived record %q missing svc attr, got %v", r.msg, r.attrs)
			}
			derivedSeen = true
		}
	}
	if !derivedSeen {
		t.Fatalf("did not observe records from derived handlers")
	}

	// Confirm the derived handlers truly shared one worker/channel/core.
	d := derived.(*NonBlockingHandler)
	g := grouped.(*NonBlockingHandler)
	if d.core != base.core || g.core != base.core {
		t.Fatalf("derived handlers do not share the base core")
	}
}

// TestNonBlockingHandler_ShutdownIdempotentAndBounded verifies Shutdown can be
// called repeatedly and returns the ctx error when the worker is permanently
// blocked rather than hanging.
func TestNonBlockingHandler_ShutdownIdempotentAndBounded(t *testing.T) {
	release := make(chan struct{})
	inner := &blockingHandler{release: release}
	h := NewNonBlockingHandler(inner, 1)

	// Fill the worker so it is stuck inside inner.Handle forever.
	_ = h.Handle(context.Background(), newRecord("stuck"))
	// Give the worker a moment to pick up the entry and block.
	time.Sleep(50 * time.Millisecond)
	_ = h.Handle(context.Background(), newRecord("queued"))

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	err := h.Shutdown(ctx)
	if err == nil {
		t.Fatalf("expected ctx deadline error from bounded Shutdown on blocked worker")
	}

	// Idempotent: a second Shutdown must not panic (double-close) and must
	// still respect its ctx bound.
	ctx2, cancel2 := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel2()
	if err := h.Shutdown(ctx2); err == nil {
		t.Fatalf("expected second bounded Shutdown to also return ctx error")
	}

	// Let the worker finish so the goroutine exits cleanly.
	close(release)
}
