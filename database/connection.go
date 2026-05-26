package database

import (
	"context"
	"database/sql"

	ddriver "github.com/godx-jp/godx-platform-framework/database/driver"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Connection is a named database connection (Laravel DB::connection(name)).
type Connection struct {
	name       string
	driverName string
	system     string
	h          ddriver.Handle
}

func newConnection(name string, h ddriver.Handle) *Connection {
	return &Connection{
		name:       name,
		driverName: h.DriverName(),
		system:     h.System(),
		h:          h,
	}
}

func (c *Connection) Name() string { return c.name }

func (c *Connection) Driver() string {
	if c == nil {
		return ""
	}
	return c.driverName
}

func (c *Connection) System() string {
	if c == nil {
		return ""
	}
	return c.system
}

func (c *Connection) Postgres() *pgxpool.Pool {
	if c == nil || c.h == nil {
		return nil
	}
	return c.h.Postgres()
}

func (c *Connection) SQL() *sql.DB {
	if c == nil || c.h == nil {
		return nil
	}
	return c.h.SQL()
}

func (c *Connection) Ping(ctx context.Context) error {
	if c == nil || c.h == nil {
		return nil
	}
	return c.h.Ping(ctx)
}

func (c *Connection) Shutdown(ctx context.Context) error {
	if c == nil || c.h == nil {
		return nil
	}
	return c.h.Shutdown(ctx)
}

func (c *Connection) handle() ddriver.Handle {
	if c == nil {
		return nil
	}
	return c.h
}
