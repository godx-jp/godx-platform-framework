package helpers

import (
	"context"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel/trace"
)

var sqlcNameRe = regexp.MustCompile(`(?m)^\s*--\s*name:\s*(\S+)`)

// ExtractSQLCName returns the sqlc query name from a SQL string, if present.
func ExtractSQLCName(sql string) string {
	m := sqlcNameRe.FindStringSubmatch(sql)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

// TracerConfig configures composite pgx query tracing.
type TracerConfig struct {
	LogQueries     bool
	SlowThreshold  time.Duration
	LogArgs        bool
	TraceQueries   bool
	Logger         *slog.Logger
	TracerProvider trace.TracerProvider
}

// NewQueryTracer builds a pgx QueryTracer from cfg.
func NewQueryTracer(cfg TracerConfig) pgx.QueryTracer {
	var tracers []pgx.QueryTracer

	if cfg.TraceQueries && cfg.TracerProvider != nil {
		tracers = append(tracers, otelpgx.NewTracer(
			otelpgx.WithTracerProvider(cfg.TracerProvider),
			otelpgx.WithTrimSQLInSpanName(),
		))
	}

	if cfg.LogQueries || cfg.SlowThreshold > 0 {
		logger := cfg.Logger
		if logger == nil {
			logger = slog.Default()
		}
		tracers = append(tracers, &queryLogger{
			logger:        logger,
			logAll:        cfg.LogQueries,
			slowThreshold: cfg.SlowThreshold,
			logArgs:       cfg.LogArgs,
		})
	}

	if len(tracers) == 0 {
		return nil
	}
	if len(tracers) == 1 {
		return tracers[0]
	}
	return &multiQueryTracer{tracers: tracers}
}

type multiQueryTracer struct {
	tracers []pgx.QueryTracer
}

func (m *multiQueryTracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	for _, t := range m.tracers {
		ctx = t.TraceQueryStart(ctx, conn, data)
	}
	return ctx
}

func (m *multiQueryTracer) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	for _, t := range m.tracers {
		t.TraceQueryEnd(ctx, conn, data)
	}
}

type queryLogger struct {
	logger        *slog.Logger
	logAll        bool
	slowThreshold time.Duration
	logArgs       bool
}

type queryLogKey struct{}

type queryLogState struct {
	start time.Time
	sql   string
	args  []any
}

func (l *queryLogger) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, queryLogKey{}, queryLogState{
		start: time.Now(),
		sql:   data.SQL,
		args:  data.Args,
	})
}

func (l *queryLogger) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	st, ok := ctx.Value(queryLogKey{}).(queryLogState)
	if !ok {
		return
	}
	d := time.Since(st.start)
	if !l.logAll && (l.slowThreshold <= 0 || d < l.slowThreshold) {
		return
	}
	attrs := []any{
		"duration_ms", d.Milliseconds(),
		"query_name", ExtractSQLCName(st.sql),
	}
	if data.Err != nil {
		attrs = append(attrs, "err", data.Err)
	}
	if l.logArgs && len(st.args) > 0 {
		attrs = append(attrs, "args", st.args)
	}
	l.logger.Log(ctx, slog.LevelInfo, "database query", attrs...)
}

// SQL operation helper for diagnostics.
func SQLOperation(stmt string) string {
	if name := ExtractSQLCName(stmt); name != "" {
		return name
	}
	fields := strings.Fields(strings.TrimSpace(stmt))
	if len(fields) == 0 {
		return "QUERY"
	}
	return strings.ToUpper(fields[0])
}
