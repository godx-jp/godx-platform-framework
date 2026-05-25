package observability

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// HTTPMetrics holds the RED (Rate, Errors, Duration) instruments for inbound
// HTTP requests, following the OpenTelemetry HTTP server semantic conventions
// (https://opentelemetry.io/docs/specs/semconv/http/http-metrics/).
//
// Error rate is derived from the duration histogram's count split by
// http.response.status_code / error.type — the OTel-idiomatic approach — so no
// separate error counter is registered.
//
// Instruments are registered once (by the HTTP middleware at startup) and
// reused for every request. When the active observability driver does not
// export metrics (stdout/file/cloudwatch use a no-op meter) recording is a
// cheap no-op, so callers never need to branch on whether metrics are enabled.
type HTTPMetrics struct {
	duration       metric.Float64Histogram
	activeRequests metric.Int64UpDownCounter
}

// NewHTTPMetrics registers the HTTP server instruments on m. A nil *HTTPMetrics
// (e.g. on registration error) is safe to use: every method becomes a no-op.
func NewHTTPMetrics(m metric.Meter) (*HTTPMetrics, error) {
	dur, err := m.Float64Histogram(
		"http.server.request.duration",
		metric.WithUnit("s"),
		metric.WithDescription("Duration of inbound HTTP server requests."),
	)
	if err != nil {
		return nil, err
	}
	active, err := m.Int64UpDownCounter(
		"http.server.active_requests",
		metric.WithUnit("{request}"),
		metric.WithDescription("Number of in-flight inbound HTTP server requests."),
	)
	if err != nil {
		return nil, err
	}
	return &HTTPMetrics{duration: dur, activeRequests: active}, nil
}

// RequestStarted increments the in-flight gauge and returns a function that
// decrements it. Defer the returned function so the gauge always settles back,
// even on panic.
func (h *HTTPMetrics) RequestStarted(ctx context.Context, method, scheme string) func() {
	if h == nil {
		return func() {}
	}
	set := metric.WithAttributes(
		attribute.String("http.request.method", method),
		attribute.String("url.scheme", scheme),
	)
	h.activeRequests.Add(ctx, 1, set)
	return func() { h.activeRequests.Add(ctx, -1, set) }
}

// RecordRequest records the request duration with RED attributes. errorType is
// recorded only when non-empty (set it for 5xx responses) so the histogram can
// be sliced into an error rate without a dedicated counter.
func (h *HTTPMetrics) RecordRequest(ctx context.Context, method, route, scheme string, status int, d time.Duration, errorType string) {
	if h == nil {
		return
	}
	attrs := []attribute.KeyValue{
		attribute.String("http.request.method", method),
		attribute.String("http.route", route),
		attribute.String("url.scheme", scheme),
		attribute.Int("http.response.status_code", status),
	}
	if errorType != "" {
		attrs = append(attrs, attribute.String("error.type", errorType))
	}
	h.duration.Record(ctx, d.Seconds(), metric.WithAttributes(attrs...))
}
