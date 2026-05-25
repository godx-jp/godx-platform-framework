package driver

import (
	"context"

	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// SamplerFor returns the recommended trace sampler for the given rate.
// Values outside (0, 1) mean AlwaysSample, which is the typical choice for
// local development and unit tests where every span should be retained.
func SamplerFor(rate float64) sdktrace.Sampler {
	if rate <= 0 || rate >= 1 {
		return sdktrace.AlwaysSample()
	}
	return sdktrace.ParentBased(sdktrace.TraceIDRatioBased(rate))
}

// ResourceFor builds the standard OTel resource for s — the
// service.name / service.version / deployment.environment trio that every
// span, metric, and log record is decorated with. Exposed so third-party
// driver implementations can share the convention.
func ResourceFor(s Spec) *sdkresource.Resource {
	r, _ := sdkresource.New(context.Background(),
		sdkresource.WithAttributes(
			semconv.ServiceName(s.ServiceName),
			semconv.ServiceVersion(s.ServiceVersion),
			semconv.DeploymentEnvironment(s.Environment),
		),
	)
	return r
}
