// Package middleware provides HTTP middleware that wires an observability
// provider into the request lifecycle: span creation, W3C trace-context
// propagation, correlation-id management, and a per-request log line.
//
// Split out of the observability package so callers that only need
// structured logging or in-process tracing do not transitively import
// net/http or commit to the framework's particular request convention.
//
//	import (
//	    "github.com/godx-jp/godx-platform-framework/observability"
//	    "github.com/godx-jp/godx-platform-framework/observability/middleware"
//	)
//
//	obs := observability.FromApp(app)
//	wrap := middleware.HTTP(obs)
//	srv := &http.Server{Handler: wrap(mux)}
package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/godx-jp/godx-platform-framework/observability"
)

// unmatchedRoute is the http.route value used when no route template is
// available (request not served through a chi router, or no route matched).
// Collapsing to a constant bounds metric/span label cardinality per the OTel
// semantic conventions.
const unmatchedRoute = "<unmatched>"

// CorrelationHeader is the HTTP header used to read or set a correlation ID
// across services. Aligned with the de-facto convention; configurable in a
// future release.
const CorrelationHeader = "X-Correlation-ID"

// HTTP returns middleware that wraps an [http.Handler] with span creation,
// correlation-id propagation, and a per-request log line. The provider is
// injected into the request context so downstream handlers can recover it
// via [observability.FromContext].
//
// Use as the outermost middleware on a router so all subsequent handlers
// inherit the correlated context.
func HTTP(p *observability.Provider) func(http.Handler) http.Handler {
	propagator := propagation.TraceContext{}
	metrics, err := observability.NewHTTPMetrics(p.Meter())
	if err != nil {
		// Degrade gracefully: tracing/logging still work without metrics.
		p.Logger().Warn("observability: HTTP metrics registration failed", "error", err)
		metrics = nil
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))

			cid := r.Header.Get(CorrelationHeader)
			if cid == "" {
				cid = newCorrelationID()
			}
			ctx = observability.ContextWithCorrelationID(ctx, cid)
			ctx = observability.ContextWithProvider(ctx, p)

			scheme := schemeOf(r)
			// Span starts before routing, so the route template is not yet
			// known; name it by method now and refine via SetName below.
			ctx, span := p.Tracer().Start(ctx, r.Method,
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(
					attribute.String("http.method", r.Method),
					attribute.String("http.target", r.URL.Path),
					attribute.String("http.scheme", scheme),
				),
			)
			defer span.End()

			w.Header().Set(CorrelationHeader, cid)
			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}

			endActive := metrics.RequestStarted(ctx, r.Method, scheme)
			start := time.Now()
			next.ServeHTTP(sw, r.WithContext(ctx))
			elapsed := time.Since(start)
			endActive()

			// The route template is populated by chi during routing; reading
			// it after ServeHTTP yields the low-cardinality pattern.
			route := routePattern(r)
			span.SetName(r.Method + " " + route)
			span.SetAttributes(
				attribute.String("http.route", route),
				attribute.Int("http.status_code", sw.status),
			)

			var errorType string
			if sw.status >= 500 {
				errorType = http.StatusText(sw.status)
				span.SetAttributes(attribute.String("error.type", errorType))
				span.SetStatus(codes.Error, errorType)
				span.RecordError(httpError{status: sw.status})
			}

			metrics.RecordRequest(ctx, r.Method, route, scheme, sw.status, elapsed, errorType)

			logAtStatus(ctx, p.Logger(), sw.status, "http_request",
				"method", r.Method,
				"path", r.URL.Path,
				"http.route", route,
				"status", sw.status,
				"duration_ms", elapsed.Milliseconds(),
				"remote", r.RemoteAddr,
				"trace_id", span.SpanContext().TraceID().String(),
			)
		})
	}
}

// routePattern returns the matched chi route template (e.g. "/users/{id}") or
// [unmatchedRoute] when the request was not served through a chi router or no
// route matched. Using the template — never the concrete path — bounds label
// cardinality.
func routePattern(r *http.Request) string {
	if rc := chi.RouteContext(r.Context()); rc != nil {
		if p := rc.RoutePattern(); p != "" {
			return p
		}
	}
	return unmatchedRoute
}

// logAtStatus emits the request log at a severity derived from the response
// status: 5xx → Error, 4xx → Warn, otherwise Info (RFC 7231 §6). This lets
// operators filter and alert on server errors by log level.
func logAtStatus(ctx context.Context, l *slog.Logger, status int, msg string, args ...any) {
	switch {
	case status >= 500:
		l.ErrorContext(ctx, msg, args...)
	case status >= 400:
		l.WarnContext(ctx, msg, args...)
	default:
		l.InfoContext(ctx, msg, args...)
	}
}

func newCorrelationID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func schemeOf(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		return proto
	}
	return "http"
}

type statusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (s *statusWriter) WriteHeader(code int) {
	if !s.wroteHeader {
		s.status = code
		s.wroteHeader = true
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusWriter) Write(p []byte) (int, error) {
	if !s.wroteHeader {
		s.WriteHeader(http.StatusOK)
	}
	return s.ResponseWriter.Write(p)
}

type httpError struct{ status int }

func (e httpError) Error() string { return http.StatusText(e.status) }
