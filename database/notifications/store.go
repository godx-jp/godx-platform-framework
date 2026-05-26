// Package notifications provides a Postgres-backed DatabaseStore for the
// notifications module database channel.
package notifications

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/godx-jp/godx-platform-framework/database"
	ndriver "github.com/godx-jp/godx-platform-framework/notifications/driver"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	EnvTable      = "DATABASE_NOTIFICATIONS_TABLE"
	EnvConnection = "DATABASE_NOTIFICATIONS_CONNECTION"
)

// StoreConfig tunes the notifications table store.
type StoreConfig struct {
	Table          string
	ConnectionName string
}

func (c StoreConfig) withDefaults() StoreConfig {
	if c.Table == "" {
		c.Table = "notifications"
	}
	if c.ConnectionName == "" {
		c.ConnectionName = "default"
	}
	return c
}

// LoadStoreConfigFromEnv reads store config from the environment.
func LoadStoreConfigFromEnv() StoreConfig {
	return StoreConfig{
		Table:          strings.TrimSpace(os.Getenv(EnvTable)),
		ConnectionName: strings.TrimSpace(os.Getenv(EnvConnection)),
	}.withDefaults()
}

// PostgresStore persists notification rows.
type PostgresStore struct {
	pool  *pgxpool.Pool
	table string
}

// NewPostgresStore returns a store backed by pool.
func NewPostgresStore(pool *pgxpool.Pool, cfg StoreConfig) (*PostgresStore, error) {
	if pool == nil {
		return nil, fmt.Errorf("database/notifications: pool is required")
	}
	cfg = cfg.withDefaults()
	return &PostgresStore{pool: pool, table: cfg.Table}, nil
}

// FromManager resolves a connection from mgr and builds a store.
func FromManager(mgr *database.Manager, cfg StoreConfig) (*PostgresStore, error) {
	if mgr == nil {
		return nil, fmt.Errorf("database/notifications: manager is required")
	}
	cfg = cfg.withDefaults()
	conn, err := mgr.Connection(cfg.ConnectionName)
	if err != nil {
		write, werr := mgr.Write()
		if werr != nil {
			return nil, err
		}
		conn = write
	}
	pool := conn.Postgres()
	if pool == nil {
		return nil, fmt.Errorf("database/notifications: connection %q is not postgres", conn.Name())
	}
	return NewPostgresStore(pool, cfg)
}

// Store implements notifications/driver.DatabaseStore.
func (s *PostgresStore) Store(ctx context.Context, record ndriver.DatabaseRecord) error {
	q := fmt.Sprintf(`INSERT INTO %s (notifiable_type, notifiable_id, channel, type, data, created_at)
VALUES ($1, $2, $3, $4, $5, now())`, s.table)
	_, err := s.pool.Exec(ctx, q,
		record.NotifiableType,
		record.NotifiableID,
		record.Channel,
		record.Type,
		record.Data,
	)
	if err != nil {
		return fmt.Errorf("database/notifications: store: %w", err)
	}
	return nil
}
