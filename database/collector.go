package database

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type metricsCollector struct {
	usage    metric.Int64Gauge
	idle     metric.Int64Gauge
	max      metric.Int64Gauge
	pending  metric.Int64Counter
	timeouts metric.Int64Counter

	lastEmpty    map[string]int64
	lastCanceled map[string]int64
}

func newMetricsCollector(m metric.Meter) (*metricsCollector, error) {
	if m == nil {
		return nil, nil
	}
	usage, err := m.Int64Gauge("db.client.connections.usage",
		metric.WithDescription("Acquired database connections"),
		metric.WithUnit("{connection}"))
	if err != nil {
		return nil, err
	}
	idle, err := m.Int64Gauge("db.client.connections.idle",
		metric.WithDescription("Idle database connections"))
	if err != nil {
		return nil, err
	}
	max, err := m.Int64Gauge("db.client.connections.max",
		metric.WithDescription("Max database connections"))
	if err != nil {
		return nil, err
	}
	pending, err := m.Int64Counter("db.client.connections.pending_requests",
		metric.WithDescription("Pool acquire waits"))
	if err != nil {
		return nil, err
	}
	timeouts, err := m.Int64Counter("db.client.connections.timeouts",
		metric.WithDescription("Canceled pool acquires"))
	if err != nil {
		return nil, err
	}
	return &metricsCollector{
		usage:        usage,
		idle:         idle,
		max:          max,
		pending:      pending,
		timeouts:     timeouts,
		lastEmpty:    map[string]int64{},
		lastCanceled: map[string]int64{},
	}, nil
}

func (c *metricsCollector) run(ctx context.Context, mgr *Manager, interval time.Duration) {
	if c == nil || mgr == nil {
		return
	}
	if interval <= 0 {
		interval = 15 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.record(ctx, mgr)
		}
	}
}

func (c *metricsCollector) record(ctx context.Context, mgr *Manager) {
	for _, name := range mgr.Connections() {
		conn, err := mgr.Connection(name)
		if err != nil {
			continue
		}
		attrs := metric.WithAttributes(
			attribute.String("db.connection.name", name),
			attribute.String("db.system", conn.System()),
		)
		if pool := conn.Postgres(); pool != nil {
			st := pool.Stat()
			c.usage.Record(ctx, int64(st.AcquiredConns()), attrs)
			c.idle.Record(ctx, int64(st.IdleConns()), attrs)
			c.max.Record(ctx, int64(st.MaxConns()), attrs)
			empty := st.EmptyAcquireCount()
			if d := empty - c.lastEmpty[name]; d > 0 {
				c.pending.Add(ctx, d, attrs)
			}
			c.lastEmpty[name] = empty
			canceled := st.CanceledAcquireCount()
			if d := canceled - c.lastCanceled[name]; d > 0 {
				c.timeouts.Add(ctx, d, attrs)
			}
			c.lastCanceled[name] = canceled
			continue
		}
		if db := conn.SQL(); db != nil {
			st := db.Stats()
			c.usage.Record(ctx, int64(st.InUse), attrs)
			c.idle.Record(ctx, int64(st.Idle), attrs)
			c.max.Record(ctx, int64(st.MaxOpenConnections), attrs)
			wait := int64(st.WaitCount)
			if d := wait - c.lastEmpty[name]; d > 0 {
				c.pending.Add(ctx, d, attrs)
			}
			c.lastEmpty[name] = wait
		}
	}
}
