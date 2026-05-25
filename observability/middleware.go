package observability

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// CorrelationHeader is the HTTP header used to read or set a correlation ID
// across services. Aligned with the de-facto convention; configurable in a
// future release.
const CorrelationHeader = "X-Correlation-ID"

// Middleware wraps an [http.Handler] with span creation, correlation-id
// propagation, and a per-request log line. It is safe to use as the outermost
// middleware on a router; downstream handlers can call [FromContext] to
// retrieve the same provider.
func (p *Provider) Middleware(next http.Handler) http.Handler {
	propagator := propagation.TraceContext{}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))

		cid := r.Header.Get(CorrelationHeader)
		if cid == "" {
			cid = newCorrelationID()
		}
		ctx = ContextWithCorrelationID(ctx, cid)
		ctx = ContextWithProvider(ctx, p)

		spanName := r.Method + " " + r.URL.Path
		ctx, span := p.tracer.Start(ctx, spanName,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				attribute.String("http.method", r.Method),
				attribute.String("http.target", r.URL.Path),
				attribute.String("http.scheme", schemeOf(r)),
			),
		)
		defer span.End()

		w.Header().Set(CorrelationHeader, cid)
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}

		start := time.Now()
		next.ServeHTTP(sw, r.WithContext(ctx))
		elapsed := time.Since(start)

		span.SetAttributes(attribute.Int("http.status_code", sw.status))
		if sw.status >= 500 {
			span.RecordError(httpError{status: sw.status})
		}

		p.logger.InfoContext(ctx, "http_request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", sw.status,
			"duration_ms", elapsed.Milliseconds(),
			"remote", r.RemoteAddr,
		)
	})
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
