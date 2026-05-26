// Package sqlite is the SQLite database driver (light — auto-registered).
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"sync"

	"github.com/XSAM/otelsql"
	ddriver "github.com/godx-jp/godx-platform-framework/database/driver"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "modernc.org/sqlite"

	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

func init() {
	ddriver.Register(ddriver.DriverSQLite, New)
}

const Name = ddriver.DriverSQLite

// New constructs a sqlite Handle from spec.
func New(ctx context.Context, spec ddriver.Spec) (ddriver.Handle, error) {
	if spec.URL == "" {
		return nil, fmt.Errorf("database/sqlite: URL is required")
	}
	var (
		db  *sql.DB
		err error
	)
	if spec.Obs.TraceQueries && spec.Obs.TracerProvider != nil {
		db, err = otelsql.Open("sqlite", spec.URL,
			otelsql.WithTracerProvider(spec.Obs.TracerProvider),
			otelsql.WithAttributes(semconv.DBSystemSqlite),
		)
	} else {
		db, err = sql.Open("sqlite", spec.URL)
	}
	if err != nil {
		return nil, fmt.Errorf("database/sqlite: open: %w", err)
	}
	db.SetMaxOpenConns(int(maxInt32(spec.MaxConns, 1)))
	db.SetMaxIdleConns(int(maxInt32(spec.MinConns, 1)))
	if spec.MaxConnIdleTime > 0 {
		db.SetConnMaxIdleTime(spec.MaxConnIdleTime)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("database/sqlite: ping: %w", err)
	}
	return &handle{db: db}, nil
}

type handle struct {
	db *sql.DB

	mu     sync.Mutex
	closed bool
}

func (h *handle) DriverName() string      { return ddriver.DriverSQLite }
func (h *handle) System() string          { return "sqlite" }
func (h *handle) Postgres() *pgxpool.Pool { return nil }
func (h *handle) SQL() *sql.DB            { return h.db }

func (h *handle) Ping(ctx context.Context) error {
	if err := h.checkOpen(); err != nil {
		return err
	}
	return h.db.PingContext(ctx)
}

func (h *handle) Shutdown(_ context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return nil
	}
	h.closed = true
	return h.db.Close()
}

func (h *handle) checkOpen() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return fmt.Errorf("database/sqlite: connection closed")
	}
	return nil
}

func maxInt32(v, def int32) int32 {
	if v > 0 {
		return v
	}
	return def
}
