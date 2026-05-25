package observability_test

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"github.com/godx-jp/godx-platform-framework/observability"
	filedrv "github.com/godx-jp/godx-platform-framework/observability/drivers/file"
)

// fileProvider builds a Provider whose primary logger writes JSON lines to a
// temp file, plus the path to read records back. The file driver uses a no-op
// meter, so these tests assert the LOG path; the counter path is exercised
// indirectly (Report must not panic when the meter is no-op) and the metric
// shape is documented rather than read back from the no-op reader.
func fileProvider(t *testing.T, level slog.Level) (*observability.Provider, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "app.log")
	p, err := observability.NewProvider(context.Background(), observability.Config{
		ServiceName:     "rep-svc",
		Driver:          observability.DriverFile,
		LogLevel:        level,
		LogFilePath:     path,
		LogFileRotation: filedrv.RotationNone,
	})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	t.Cleanup(func() { _ = p.Shutdown(context.Background()) })
	return p, path
}

func readRecords(t *testing.T, p *observability.Provider, path string) []map[string]any {
	t.Helper()
	if err := p.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer f.Close()
	var recs []map[string]any
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec map[string]any
		if err := json.Unmarshal(line, &rec); err != nil {
			t.Fatalf("decode %q: %v", string(line), err)
		}
		recs = append(recs, rec)
	}
	return recs
}

// --- fakes ----------------------------------------------------------------

type countingNotifier struct {
	n        int64
	failNext atomic.Bool
}

func (c *countingNotifier) Notify(ctx context.Context, subject, body string) error {
	atomic.AddInt64(&c.n, 1)
	if c.failNext.Load() {
		return errors.New("boom")
	}
	return nil
}

func (c *countingNotifier) count() int64 { return atomic.LoadInt64(&c.n) }

// blockingNotifier blocks each Notify until its context expires, simulating a
// slow/hung alerting endpoint.
type blockingNotifier struct{ calls atomic.Int64 }

func (b *blockingNotifier) Notify(ctx context.Context, _, _ string) error {
	b.calls.Add(1)
	<-ctx.Done()
	return ctx.Err()
}

// drain stops the reporter's async notify worker, processing queued alerts, so
// notification assertions are deterministic.
func drain(t *testing.T, rep *observability.Reporter) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := rep.Shutdown(ctx); err != nil {
		t.Fatalf("reporter Shutdown: %v", err)
	}
}

// recordingReporter captures reports for adapter unit tests without needing a
// real Provider/meter.
type recordingReporter struct {
	mu      sync.Mutex
	reports []observability.ErrorReport
}

func (r *recordingReporter) Report(_ context.Context, rep observability.ErrorReport) {
	r.mu.Lock()
	r.reports = append(r.reports, rep)
	r.mu.Unlock()
}

func (r *recordingReporter) all() []observability.ErrorReport {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]observability.ErrorReport, len(r.reports))
	copy(out, r.reports)
	return out
}

// --- log path -------------------------------------------------------------

func TestReport_LogsAtSeverityWithAttrs(t *testing.T) {
	p, path := fileProvider(t, slog.LevelInfo)
	rep := observability.NewReporter(p, observability.ReporterOptions{})

	rep.Report(context.Background(), observability.ErrorReport{
		Err:      errors.New("disk full"),
		Source:   "queue",
		Severity: slog.LevelError,
		Attrs:    []slog.Attr{slog.String("subject", "orders.created")},
	})

	recs := readRecords(t, p, path)
	if len(recs) != 1 {
		t.Fatalf("want 1 record, got %d: %+v", len(recs), recs)
	}
	rec := recs[0]
	if rec["level"] != "ERROR" {
		t.Errorf("level = %v, want ERROR", rec["level"])
	}
	if rec["msg"] != "disk full" {
		t.Errorf("msg = %v, want 'disk full'", rec["msg"])
	}
	if rec["source"] != "queue" {
		t.Errorf("source = %v, want queue", rec["source"])
	}
	if rec["error.type"] != "*errors.errorString" {
		t.Errorf("error.type = %v, want *errors.errorString", rec["error.type"])
	}
	if rec["subject"] != "orders.created" {
		t.Errorf("subject = %v, want orders.created", rec["subject"])
	}
}

func TestReport_ZeroSeverityDefaultsToError(t *testing.T) {
	p, path := fileProvider(t, slog.LevelInfo)
	rep := observability.NewReporter(p, observability.ReporterOptions{})

	rep.Report(context.Background(), observability.ErrorReport{
		Err:    errors.New("boom"),
		Source: "events",
	})

	recs := readRecords(t, p, path)
	if len(recs) != 1 || recs[0]["level"] != "ERROR" {
		t.Fatalf("want one ERROR record, got %+v", recs)
	}
}

// --- adapter severity mapping --------------------------------------------

func TestHTTPErrorObserver_SeverityMapping(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		want   *slog.Level // nil => expect no report
	}{
		{"500 -> Error", errors.New("x"), 500, levelPtr(slog.LevelError)},
		{"503 -> Error", errors.New("x"), 503, levelPtr(slog.LevelError)},
		{"404 -> Warn", errors.New("x"), 404, levelPtr(slog.LevelWarn)},
		{"400 -> Warn", errors.New("x"), 400, levelPtr(slog.LevelWarn)},
		{"200 -> Info", errors.New("x"), 200, levelPtr(slog.LevelInfo)},
		{"nil err skipped (500)", nil, 500, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := &recordingReporter{}
			obs := observability.HTTPErrorObserver(rr)
			obs(context.Background(), tt.err, tt.status)

			got := rr.all()
			if tt.want == nil {
				if len(got) != 0 {
					t.Fatalf("expected no report, got %+v", got)
				}
				return
			}
			if len(got) != 1 {
				t.Fatalf("want 1 report, got %d", len(got))
			}
			if got[0].Severity != *tt.want {
				t.Errorf("severity = %v, want %v", got[0].Severity, *tt.want)
			}
			if got[0].Source != "http" {
				t.Errorf("source = %q, want http", got[0].Source)
			}
		})
	}
}

func TestErrorHook_SkipsNilAndSetsSource(t *testing.T) {
	rr := &recordingReporter{}
	hook := observability.ErrorHook(rr, "outbox")

	hook(nil)
	if got := rr.all(); len(got) != 0 {
		t.Fatalf("nil err should be skipped, got %+v", got)
	}

	hook(errors.New("relay failed"))
	got := rr.all()
	if len(got) != 1 {
		t.Fatalf("want 1 report, got %d", len(got))
	}
	if got[0].Source != "outbox" {
		t.Errorf("source = %q, want outbox", got[0].Source)
	}
}

func TestJobErrorHook_SkipsNilAndAttachesJob(t *testing.T) {
	rr := &recordingReporter{}
	hook := observability.JobErrorHook(rr, "scheduler")

	hook("nightly-rollup", nil)
	if got := rr.all(); len(got) != 0 {
		t.Fatalf("nil err should be skipped, got %+v", got)
	}

	hook("nightly-rollup", errors.New("timeout"))
	got := rr.all()
	if len(got) != 1 {
		t.Fatalf("want 1 report, got %d", len(got))
	}
	if got[0].Source != "scheduler" {
		t.Errorf("source = %q, want scheduler", got[0].Source)
	}
	var foundJob bool
	for _, a := range got[0].Attrs {
		if a.Key == "job" && a.Value.String() == "nightly-rollup" {
			foundJob = true
		}
	}
	if !foundJob {
		t.Errorf("expected job=nightly-rollup attr, got %+v", got[0].Attrs)
	}
}

// --- rate limiter ---------------------------------------------------------

func TestReport_RateLimitsNotifications(t *testing.T) {
	p, _ := fileProvider(t, slog.LevelError)
	notifier := &countingNotifier{}
	rep := observability.NewReporter(p, observability.ReporterOptions{
		Notifier:    notifier,
		NotifyRate:  rate.Limit(0), // effectively no refill within the test
		NotifyBurst: 1,
	})

	const fired = 100
	for i := 0; i < fired; i++ {
		rep.Report(context.Background(), observability.ErrorReport{
			Err:      errors.New("flood"),
			Source:   "queue",
			Severity: slog.LevelError,
		})
	}
	drain(t, rep)
	// burst=1, no refill => exactly one notification gets through.
	if got := notifier.count(); got != 1 {
		t.Fatalf("notifications = %d, want 1 (rate-limited from %d reports)", got, fired)
	}
}

func TestReport_NotifyRespectsMinLevel(t *testing.T) {
	p, _ := fileProvider(t, slog.LevelDebug)
	notifier := &countingNotifier{}
	rep := observability.NewReporter(p, observability.ReporterOptions{
		Notifier:       notifier,
		NotifyMinLevel: slog.LevelError,
		NotifyRate:     rate.Inf, // never the limiting factor here
		NotifyBurst:    1000,
	})

	// Below threshold: no notification.
	rep.Report(context.Background(), observability.ErrorReport{
		Err: errors.New("warn-only"), Source: "http", Severity: slog.LevelWarn,
	})
	if got := notifier.count(); got != 0 {
		t.Fatalf("warn report should not notify, got %d", got)
	}

	// At threshold: notifies.
	rep.Report(context.Background(), observability.ErrorReport{
		Err: errors.New("server"), Source: "http", Severity: slog.LevelError,
	})
	drain(t, rep)
	if got := notifier.count(); got != 1 {
		t.Fatalf("error report should notify, got %d", got)
	}
}

func TestReport_DistinctKeysHaveDistinctBuckets(t *testing.T) {
	p, _ := fileProvider(t, slog.LevelError)
	notifier := &countingNotifier{}
	rep := observability.NewReporter(p, observability.ReporterOptions{
		Notifier:    notifier,
		NotifyRate:  rate.Limit(0),
		NotifyBurst: 1,
	})

	// Two different sources => two buckets => two notifications get through.
	rep.Report(context.Background(), observability.ErrorReport{Err: errors.New("a"), Source: "queue", Severity: slog.LevelError})
	rep.Report(context.Background(), observability.ErrorReport{Err: errors.New("a"), Source: "queue", Severity: slog.LevelError})
	rep.Report(context.Background(), observability.ErrorReport{Err: errors.New("a"), Source: "events", Severity: slog.LevelError})

	drain(t, rep)
	if got := notifier.count(); got != 2 {
		t.Fatalf("notifications = %d, want 2 (one per distinct source)", got)
	}
}

// --- notifier error swallowed --------------------------------------------

func TestReport_NotifierErrorSwallowed(t *testing.T) {
	p, _ := fileProvider(t, slog.LevelError)
	notifier := &countingNotifier{}
	notifier.failNext.Store(true)
	rep := observability.NewReporter(p, observability.ReporterOptions{
		Notifier:    notifier,
		NotifyRate:  rate.Inf,
		NotifyBurst: 10,
	})

	// Must not panic or block; Report returns normally despite notifier error.
	rep.Report(context.Background(), observability.ErrorReport{
		Err: errors.New("x"), Source: "http", Severity: slog.LevelError,
	})
	drain(t, rep)
	if got := notifier.count(); got != 1 {
		t.Fatalf("notifier should have been called once, got %d", got)
	}
}

// --- nil provider ---------------------------------------------------------

func TestNewReporter_NilProviderFallsBack(t *testing.T) {
	rep := observability.NewReporter(nil, observability.ReporterOptions{})
	// Should not panic.
	rep.Report(context.Background(), observability.ErrorReport{
		Err: errors.New("x"), Source: "events",
	})
}

// --- concurrency / race ---------------------------------------------------

func TestReport_ConcurrentSafe(t *testing.T) {
	p, _ := fileProvider(t, slog.LevelError)
	notifier := &countingNotifier{}
	rep := observability.NewReporter(p, observability.ReporterOptions{
		Notifier:    notifier,
		NotifyRate:  rate.Limit(0),
		NotifyBurst: 1,
	})

	var wg sync.WaitGroup
	const goroutines = 16
	const each = 50
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < each; i++ {
				rep.Report(context.Background(), observability.ErrorReport{
					Err:      errors.New("concurrent"),
					Source:   "queue",
					Severity: slog.LevelError,
				})
			}
		}(g)
	}
	wg.Wait()

	drain(t, rep)
	// All share the same key (queue+*errors.errorString), burst=1, no refill =>
	// at most one notification regardless of goroutine interleaving.
	if got := notifier.count(); got != 1 {
		t.Fatalf("notifications = %d, want exactly 1 under concurrency", got)
	}
}

// TestReport_SlowNotifierDoesNotBlock proves a hung alerting endpoint cannot
// block the caller (the request goroutine): Report returns promptly and excess
// alerts are dropped (counted) rather than queued unboundedly.
func TestReport_SlowNotifierDoesNotBlock(t *testing.T) {
	p, _ := fileProvider(t, slog.LevelError)
	rep := observability.NewReporter(p, observability.ReporterOptions{
		Notifier:      &blockingNotifier{},
		NotifyRate:    rate.Inf, // rate limiter never the gate here
		NotifyBurst:   1_000_000,
		NotifyBuffer:  1,
		NotifyTimeout: 20 * time.Millisecond,
	})

	const fired = 200
	start := time.Now()
	for i := 0; i < fired; i++ {
		rep.Report(context.Background(), observability.ErrorReport{
			Err:      errors.New("slow sink"),
			Source:   "http",
			Severity: slog.LevelError,
		})
	}
	elapsed := time.Since(start)

	// 200 reports against a notifier that blocks 20ms each: if Report were
	// synchronous this would take >4s. Non-blocking enqueue must finish fast.
	if elapsed > time.Second {
		t.Fatalf("Report blocked on slow notifier: %d reports took %v", fired, elapsed)
	}
	drain(t, rep)
	if rep.NotificationsDropped() == 0 {
		t.Fatalf("expected some notifications dropped (buffer=1, slow sink), got 0")
	}
}

func levelPtr(l slog.Level) *slog.Level { return &l }
