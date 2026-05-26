// Package driver is the public contract for database backend implementations.
package driver

import (
	"context"
	"database/sql"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Handle is a live connection pool returned by a driver constructor.
type Handle interface {
	Ping(ctx context.Context) error
	Shutdown(ctx context.Context) error
	Postgres() *pgxpool.Pool
	SQL() *sql.DB
	System() string
	DriverName() string
}

// Constructor builds a Handle from Spec.
type Constructor func(ctx context.Context, spec Spec) (Handle, error)
