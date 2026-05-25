// Package middleware provides HTTP RoundTripper wrappers for the
// httpclient module — primarily OTel client-span instrumentation.
package middleware

import (
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// Tracer is reserved for future explicit tracer injection.
type Tracer interface {
	trace.Tracer
}

// InstrumentTransport wraps base with OTel client spans. serviceName
// labels the tracer (defaults to "httpclient" when empty).
func InstrumentTransport(base http.RoundTripper, serviceName string) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	if serviceName == "" {
		serviceName = "httpclient"
	}
	tr := otel.Tracer(serviceName)
	prop := propagation.TraceContext{}
	return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		ctx := req.Context()
		spanName := req.Method + " " + req.URL.Host + req.URL.Path
		ctx, span := tr.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
		defer span.End()
		span.SetAttributes(
			semconv.HTTPRequestMethodKey.String(req.Method),
			semconv.URLFull(req.URL.String()),
		)
		prop.Inject(ctx, propagation.HeaderCarrier(req.Header))
		req = req.WithContext(ctx)
		resp, err := base.RoundTrip(req)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return resp, err
		}
		span.SetAttributes(attribute.Int("http.status_code", resp.StatusCode))
		if resp.StatusCode >= 500 {
			span.SetStatus(codes.Error, resp.Status)
		}
		return resp, err
	})
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
