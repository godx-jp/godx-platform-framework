package drivers

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
)

// otlpDriver exports traces and metrics over OTLP (gRPC or HTTP) and writes
// logs as JSON to stdout. Log shipping is handled out-of-process (Promtail,
// Fluent Bit, OTel Collector filelog receiver). This keeps the in-process
// dependency surface small and aligns with the standard Loki / Datadog
// container-log workflow.
type otlpDriver struct {
	handler slog.Handler
	tp      *sdktrace.TracerProvider
	mp      *sdkmetric.MeterProvider
}

func newOTLP(ctx context.Context, s Spec) (*otlpDriver, error) {
	traceExp, err := newTraceExporter(ctx, s)
	if err != nil {
		return nil, fmt.Errorf("otlp trace exporter: %w", err)
	}
	metricExp, err := newMetricExporter(ctx, s)
	if err != nil {
		return nil, fmt.Errorf("otlp metric exporter: %w", err)
	}

	res := resourceFor(s)

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExp),
		sdktrace.WithSampler(samplerFor(s.TraceSampleRate)),
		sdktrace.WithResource(res),
	)
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExp)),
		sdkmetric.WithResource(res),
	)

	return &otlpDriver{
		handler: slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: s.LogLevel}),
		tp:      tp,
		mp:      mp,
	}, nil
}

func (d *otlpDriver) LoggerHandler() slog.Handler          { return d.handler }
func (d *otlpDriver) TracerProvider() trace.TracerProvider { return d.tp }
func (d *otlpDriver) MeterProvider() metric.MeterProvider  { return d.mp }

func (d *otlpDriver) Shutdown(ctx context.Context) error {
	if err := d.tp.Shutdown(ctx); err != nil {
		return err
	}
	return d.mp.Shutdown(ctx)
}

func newTraceExporter(ctx context.Context, s Spec) (sdktrace.SpanExporter, error) {
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

func newMetricExporter(ctx context.Context, s Spec) (sdkmetric.Exporter, error) {
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
