// Package otlp ships traces and metrics over OTLP (gRPC or HTTP) to any
// compatible backend — godx-platform-observability, Datadog Agent, New
// Relic, Honeycomb, Grafana Cloud, etc. Logs continue to write to stdout so
// container orchestrators or log collectors can ship them alongside the
// rest of stdout.
//
// Heavy driver (~20MB of OTel exporter dependencies). NOT auto-registered;
// add an explicit blank import to enable:
//
//	import _ "github.com/godx-jp/godx-platform-framework/observability/drivers/otlp"
package otlp

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"github.com/godx-jp/godx-platform-framework/observability/driver"
)

// Name is the identifier used by OBSERVABILITY_DRIVER to select this driver.
const Name = "otlp"

func init() { driver.Register(Name, New) }

// New constructs the OTLP driver from spec. Returns an error if the configured
// endpoint cannot establish an exporter.
func New(ctx context.Context, s driver.Spec) (driver.Driver, error) {
	traceExp, err := newTraceExporter(ctx, s)
	if err != nil {
		return nil, fmt.Errorf("otlp trace exporter: %w", err)
	}
	metricExp, err := newMetricExporter(ctx, s)
	if err != nil {
		return nil, fmt.Errorf("otlp metric exporter: %w", err)
	}

	res := driver.ResourceFor(s)
	return &impl{
		handler: slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: s.LogLevel}),
		tp: sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(traceExp),
			sdktrace.WithSampler(driver.SamplerFor(s.TraceSampleRate)),
			sdktrace.WithResource(res),
		),
		mp: sdkmetric.NewMeterProvider(
			sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExp)),
			sdkmetric.WithResource(res),
		),
	}, nil
}

type impl struct {
	handler slog.Handler
	tp      *sdktrace.TracerProvider
	mp      *sdkmetric.MeterProvider
}

func (d *impl) LoggerHandler() slog.Handler          { return d.handler }
func (d *impl) TracerProvider() trace.TracerProvider { return d.tp }
func (d *impl) MeterProvider() metric.MeterProvider  { return d.mp }

func (d *impl) Shutdown(ctx context.Context) error {
	if err := d.tp.Shutdown(ctx); err != nil {
		return err
	}
	return d.mp.Shutdown(ctx)
}

func newTraceExporter(ctx context.Context, s driver.Spec) (sdktrace.SpanExporter, error) {
	switch s.OTLPProtocol {
	case "http", "http/protobuf":
		opts := []otlptracehttp.Option{otlptracehttp.WithEndpoint(s.OTLPEndpoint)}
		if s.OTLPInsecure {
			opts = append(opts, otlptracehttp.WithInsecure())
		}
		return otlptrace.New(ctx, otlptracehttp.NewClient(opts...))
	default: // "grpc"
		opts := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(s.OTLPEndpoint)}
		if s.OTLPInsecure {
			opts = append(opts, otlptracegrpc.WithInsecure())
		}
		return otlptracegrpc.New(ctx, opts...)
	}
}

func newMetricExporter(ctx context.Context, s driver.Spec) (sdkmetric.Exporter, error) {
	switch s.OTLPProtocol {
	case "http", "http/protobuf":
		opts := []otlpmetrichttp.Option{otlpmetrichttp.WithEndpoint(s.OTLPEndpoint)}
		if s.OTLPInsecure {
			opts = append(opts, otlpmetrichttp.WithInsecure())
		}
		return otlpmetrichttp.New(ctx, opts...)
	default:
		opts := []otlpmetricgrpc.Option{otlpmetricgrpc.WithEndpoint(s.OTLPEndpoint)}
		if s.OTLPInsecure {
			opts = append(opts, otlpmetricgrpc.WithInsecure())
		}
		return otlpmetricgrpc.New(ctx, opts...)
	}
}
