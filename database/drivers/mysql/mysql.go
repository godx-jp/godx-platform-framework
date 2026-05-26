// Package mysql is the MySQL database driver (heavy — blank import required).
package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/XSAM/otelsql"
	_ "github.com/go-sql-driver/mysql"
	ddriver "github.com/godx-jp/godx-platform-framework/database/driver"
	"github.com/jackc/pgx/v5/pgxpool"

	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

func init() {
	ddriver.Register(ddriver.DriverMySQL, New)
}

const Name = ddriver.DriverMySQL

// New constructs a mysql Handle from spec.
func New(ctx context.Context, spec ddriver.Spec) (ddriver.Handle, error) {
	if spec.URL == "" {
		return nil, fmt.Errorf("database/mysql: URL is required")
	}
	var (
		db  *sql.DB
		err error
	)
	if spec.Obs.TraceQueries && spec.Obs.TracerProvider != nil {
		db, err = otelsql.Open("mysql", spec.URL,
			otelsql.WithTracerProvider(spec.Obs.TracerProvider),
			otelsql.WithAttributes(semconv.DBSystemMySQL),
		)
	} else {
		db, err = sql.Open("mysql", spec.URL)
	}
	if err != nil {
		return nil, fmt.Errorf("database/mysql: open: %w", err)
	}
	db.SetMaxOpenConns(int(maxInt32(spec.MaxConns, 25)))
	db.SetMaxIdleConns(int(maxInt32(spec.MinConns, 5)))
	if spec.MaxConnLifetime > 0 {
		db.SetConnMaxLifetime(spec.MaxConnLifetime)
	} else {
		db.SetConnMaxLifetime(time.Hour)
	}
	if spec.MaxConnIdleTime > 0 {
		db.SetConnMaxIdleTime(spec.MaxConnIdleTime)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("database/mysql: ping: %w", err)
	}
	return &handle{db: db}, nil
}

type handle struct {
	db *sql.DB

	mu     sync.Mutex
	closed bool
}

func (h *handle) DriverName() string      { return ddriver.DriverMySQL }
func (h *handle) System() string          { return "mysql" }
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
		return fmt.Errorf("database/mysql: connection closed")
	}
	return nil
}

func maxInt32(v, def int32) int32 {
	if v > 0 {
		return v
	}
	return def
}
