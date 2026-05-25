package cloudwatch

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"

	"github.com/godx-jp/godx-platform-framework/observability/driver"
)

func TestRegisteredOnImport(t *testing.T) {
	if _, ok := driver.Lookup(Name); !ok {
		t.Fatal("cloudwatch driver not registered")
	}
}

func TestNewRequiresLogGroup(t *testing.T) {
	_, err := New(context.Background(), driver.Spec{Name: Name, AWSRegion: "us-east-1"})
	if err == nil {
		t.Fatal("expected error for missing log group")
	}
}

func TestDefaultStreamName(t *testing.T) {
	got := defaultStreamName("mysvc")
	if got == "" || !strings.Contains(got, "mysvc") {
		t.Fatalf("stream=%q", got)
	}
}

func TestCWHandlerBatching(t *testing.T) {
	fake := &fakeLogsClient{}
	h := &cwHandler{
		client:     fake,
		group:      "/test/app",
		stream:     "svc/host",
		level:      slog.LevelInfo,
		flushEvery: time.Hour,
		maxBatch:   2,
		lastFlush:  time.Now(),
	}
	h.inner = slog.NewJSONHandler(&lineBuffer{h: h}, &slog.HandlerOptions{Level: slog.LevelInfo})

	logger := slog.New(h)
	logger.Info("one")
	logger.Info("two")
	if fake.puts != 1 {
		t.Fatalf("puts=%d want 1 after batch fill", fake.puts)
	}
	logger.Info("three")
	if err := h.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	if fake.puts != 2 {
		t.Fatalf("puts=%d want 2 after manual flush", fake.puts)
	}
}

type fakeLogsClient struct {
	puts int
}

func (f *fakeLogsClient) PutLogEvents(_ context.Context, in *cloudwatchlogs.PutLogEventsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error) {
	f.puts++
	if len(in.LogEvents) == 0 {
		return nil, nil
	}
	return &cloudwatchlogs.PutLogEventsOutput{NextSequenceToken: aws.String("next")}, nil
}
