// Package cloudwatch ships structured JSON logs to AWS CloudWatch Logs via
// aws-sdk-go-v2. Traces remain in-process (like the file driver); metrics
// are no-op until ADOT export lands.
//
// Heavy driver — import via:
//
//	import _ "github.com/godx-jp/godx-platform-framework/observability/drivers/cloudwatch"
package cloudwatch

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwltypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"github.com/godx-jp/godx-platform-framework/observability/driver"
)

const Name = "cloudwatch"

func init() { driver.Register(Name, New) }

// New constructs the CloudWatch Logs driver.
func New(ctx context.Context, s driver.Spec) (driver.Driver, error) {
	group := strings.TrimSpace(s.CloudWatchLogGroup)
	if group == "" {
		return nil, fmt.Errorf("cloudwatch driver: CloudWatchLogGroup required (set OBSERVABILITY_CLOUDWATCH_LOG_GROUP)")
	}

	var loadOpts []func(*awsconfig.LoadOptions) error
	if region := strings.TrimSpace(s.AWSRegion); region != "" {
		loadOpts = append(loadOpts, awsconfig.WithRegion(region))
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("cloudwatch driver: load aws config: %w", err)
	}

	stream := defaultStreamName(s.ServiceName)
	client := cloudwatchlogs.NewFromConfig(cfg)
	if err := ensureLogGroupStream(ctx, client, group, stream); err != nil {
		return nil, err
	}

	h := newCWHandler(client, group, stream, s.LogLevel)
	d := &impl{
		handler: h,
		tp: sdktrace.NewTracerProvider(
			sdktrace.WithSampler(driver.SamplerFor(s.TraceSampleRate)),
			sdktrace.WithResource(driver.ResourceFor(s)),
		),
		flush: h.Flush,
	}
	return d, nil
}

type impl struct {
	handler slog.Handler
	tp      *sdktrace.TracerProvider
	flush   func(context.Context) error
}

func (d *impl) LoggerHandler() slog.Handler          { return d.handler }
func (d *impl) TracerProvider() trace.TracerProvider { return d.tp }
func (d *impl) MeterProvider() metric.MeterProvider  { return metricnoop.NewMeterProvider() }

func (d *impl) Shutdown(ctx context.Context) error {
	if d.flush != nil {
		_ = d.flush(ctx)
	}
	return d.tp.Shutdown(ctx)
}

func defaultStreamName(service string) string {
	if service == "" {
		service = "app"
	}
	host, _ := os.Hostname()
	if host == "" {
		host = "local"
	}
	return fmt.Sprintf("%s/%s", service, host)
}

func ensureLogGroupStream(ctx context.Context, client *cloudwatchlogs.Client, group, stream string) error {
	_, err := client.CreateLogGroup(ctx, &cloudwatchlogs.CreateLogGroupInput{
		LogGroupName: aws.String(group),
	})
	if err != nil && !isAlreadyExists(err) {
		return fmt.Errorf("cloudwatch driver: create log group: %w", err)
	}
	_, err = client.CreateLogStream(ctx, &cloudwatchlogs.CreateLogStreamInput{
		LogGroupName:  aws.String(group),
		LogStreamName: aws.String(stream),
	})
	if err != nil && !isAlreadyExists(err) {
		return fmt.Errorf("cloudwatch driver: create log stream: %w", err)
	}
	return nil
}

func isAlreadyExists(err error) bool {
	var exists *cwltypes.ResourceAlreadyExistsException
	return errors.As(err, &exists)
}

type logsAPI interface {
	PutLogEvents(ctx context.Context, params *cloudwatchlogs.PutLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error)
}

// cwHandler batches JSON log lines to CloudWatch Logs.
type cwHandler struct {
	client logsAPI
	group  string
	stream string
	level  slog.Level

	mu           sync.Mutex
	events       []cwltypes.InputLogEvent
	seq          *string
	closed       bool
	flushEvery   time.Duration
	maxBatch     int
	lastFlush    time.Time
	inner        slog.Handler
}

func newCWHandler(client logsAPI, group, stream string, level slog.Level) *cwHandler {
	h := &cwHandler{
		client:     client,
		group:      group,
		stream:     stream,
		level:      level,
		flushEvery: 2 * time.Second,
		maxBatch:   100,
		lastFlush:  time.Now(),
	}
	h.inner = slog.NewJSONHandler(&lineBuffer{h: h}, &slog.HandlerOptions{Level: level})
	return h
}

type lineBuffer struct {
	h *cwHandler
}

func (b *lineBuffer) Write(p []byte) (int, error) {
	b.h.appendLine(p)
	return len(p), nil
}

func (h *cwHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *cwHandler) Handle(ctx context.Context, r slog.Record) error {
	return h.inner.Handle(ctx, r)
}

func (h *cwHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clone := *h
	clone.inner = h.inner.WithAttrs(attrs)
	return &clone
}

func (h *cwHandler) WithGroup(name string) slog.Handler {
	clone := *h
	clone.inner = h.inner.WithGroup(name)
	return &clone
}

func (h *cwHandler) appendLine(line []byte) {
	line = append([]byte(nil), line...)
	ts := time.Now().UnixMilli()
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	h.events = append(h.events, cwltypes.InputLogEvent{
		Message:   aws.String(string(line)),
		Timestamp: aws.Int64(ts),
	})
	if len(h.events) >= h.maxBatch || time.Since(h.lastFlush) >= h.flushEvery {
		_ = h.flushLocked(context.Background())
	}
}

func (h *cwHandler) Flush(ctx context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.flushLocked(ctx)
}

func (h *cwHandler) flushLocked(ctx context.Context) error {
	if len(h.events) == 0 {
		h.lastFlush = time.Now()
		return nil
	}
	out, err := h.client.PutLogEvents(ctx, &cloudwatchlogs.PutLogEventsInput{
		LogGroupName:  aws.String(h.group),
		LogStreamName: aws.String(h.stream),
		LogEvents:     h.events,
		SequenceToken: h.seq,
	})
	if err != nil {
		return err
	}
	h.seq = out.NextSequenceToken
	h.events = h.events[:0]
	h.lastFlush = time.Now()
	return nil
}
