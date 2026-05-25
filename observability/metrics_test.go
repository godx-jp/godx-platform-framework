package observability_test

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/godx-jp/godx-platform-framework/observability"
)

func TestHTTPMetrics_RecordsDurationAndActive(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { _ = mp.Shutdown(context.Background()) })

	m, err := observability.NewHTTPMetrics(mp.Meter("test"))
	if err != nil {
		t.Fatalf("NewHTTPMetrics: %v", err)
	}

	ctx := context.Background()
	end := m.RequestStarted(ctx, "GET", "http")
	m.RecordRequest(ctx, "GET", "/users/{id}", "http", 500, 12*time.Millisecond, "Internal Server Error")
	end()

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("collect: %v", err)
	}

	var sawDuration, sawActive bool
	for _, sm := range rm.ScopeMetrics {
		for _, md := range sm.Metrics {
			switch md.Name {
			case "http.server.request.duration":
				sawDuration = true
				h, ok := md.Data.(metricdata.Histogram[float64])
				if !ok || len(h.DataPoints) != 1 {
					t.Fatalf("duration: unexpected data %T", md.Data)
				}
				dp := h.DataPoints[0]
				if dp.Count != 1 {
					t.Errorf("duration count = %d, want 1", dp.Count)
				}
				assertAttrStr(t, dp.Attributes, "http.route", "/users/{id}")
				assertAttrStr(t, dp.Attributes, "http.request.method", "GET")
				assertAttrStr(t, dp.Attributes, "error.type", "Internal Server Error")
				assertAttrInt(t, dp.Attributes, "http.response.status_code", 500)
			case "http.server.active_requests":
				sawActive = true
				s, ok := md.Data.(metricdata.Sum[int64])
				if !ok || len(s.DataPoints) != 1 {
					t.Fatalf("active: unexpected data %T", md.Data)
				}
				if got := s.DataPoints[0].Value; got != 0 {
					t.Errorf("active_requests settled at %d, want 0", got)
				}
			}
		}
	}
	if !sawDuration {
		t.Error("http.server.request.duration not recorded")
	}
	if !sawActive {
		t.Error("http.server.active_requests not recorded")
	}
}

// TestHTTPMetrics_NilSafe ensures a nil *HTTPMetrics (registration failure path)
// never panics.
func TestHTTPMetrics_NilSafe(t *testing.T) {
	var m *observability.HTTPMetrics
	end := m.RequestStarted(context.Background(), "GET", "http")
	end()
	m.RecordRequest(context.Background(), "GET", "/x", "http", 200, time.Millisecond, "")
}

func assertAttrStr(t *testing.T, set attribute.Set, key, want string) {
	t.Helper()
	v, ok := set.Value(attribute.Key(key))
	if !ok || v.AsString() != want {
		t.Errorf("attr %q = %q (present=%v), want %q", key, v.AsString(), ok, want)
	}
}

func assertAttrInt(t *testing.T, set attribute.Set, key string, want int64) {
	t.Helper()
	v, ok := set.Value(attribute.Key(key))
	if !ok || v.AsInt64() != want {
		t.Errorf("attr %q = %d (present=%v), want %d", key, v.AsInt64(), ok, want)
	}
}
