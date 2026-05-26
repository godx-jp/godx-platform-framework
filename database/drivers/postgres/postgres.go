// Package postgres is the pgx/v5 database driver (heavy — blank import required).
package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"sync"

	ddriver "github.com/godx-jp/godx-platform-framework/database/driver"
	"github.com/godx-jp/godx-platform-framework/database/helpers"
	"github.com/jackc/pgx/v5/pgxpool"
)

func init() {
	ddriver.Register(ddriver.DriverPostgres, New)
}

const Name = ddriver.DriverPostgres

// New constructs a postgres Handle from spec.
func New(ctx context.Context, spec ddriver.Spec) (ddriver.Handle, error) {
	if spec.URL == "" {
		return nil, fmt.Errorf("database/postgres: URL is required")
	}
	cfg, err := pgxpool.ParseConfig(spec.URL)
	if err != nil {
		return nil, fmt.Errorf("database/postgres: parse url: %w", err)
	}
	if spec.MaxConns > 0 {
		cfg.MaxConns = spec.MaxConns
	} else if cfg.MaxConns == 0 {
		cfg.MaxConns = 25
	}
	if spec.MinConns > 0 {
		cfg.MinConns = spec.MinConns
	} else if cfg.MinConns == 0 {
		cfg.MinConns = 5
	}
	if spec.MaxConnLifetime > 0 {
		cfg.MaxConnLifetime = spec.MaxConnLifetime
	}
	if spec.MaxConnIdleTime > 0 {
		cfg.MaxConnIdleTime = spec.MaxConnIdleTime
	}
	if spec.HealthCheckPeriod > 0 {
		cfg.HealthCheckPeriod = spec.HealthCheckPeriod
	}

	tracer := helpers.NewQueryTracer(helpers.TracerConfig{
		LogQueries:     spec.Obs.LogQueries,
		SlowThreshold:  spec.Obs.SlowThreshold,
		LogArgs:        spec.Obs.LogArgs,
		TraceQueries:   spec.Obs.TraceQueries,
		Logger:         spec.Obs.Logger,
		TracerProvider: spec.Obs.TracerProvider,
	})
	if tracer != nil {
		cfg.ConnConfig.Tracer = tracer
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("database/postgres: create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database/postgres: ping: %w", err)
	}
	return &handle{pool: pool}, nil
}

type handle struct {
	pool *pgxpool.Pool

	mu     sync.Mutex
	closed bool
}

func (h *handle) DriverName() string { return ddriver.DriverPostgres }
func (h *handle) System() string     { return "postgresql" }
func (h *handle) Postgres() *pgxpool.Pool {
	return h.pool
}
func (h *handle) SQL() *sql.DB { return nil }

func (h *handle) Ping(ctx context.Context) error {
	if err := h.checkOpen(); err != nil {
		return err
	}
	return h.pool.Ping(ctx)
}

func (h *handle) Shutdown(_ context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return nil
	}
	h.closed = true
	h.pool.Close()
	return nil
}

func (h *handle) checkOpen() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return fmt.Errorf("database/postgres: connection closed")
	}
	return nil
}
