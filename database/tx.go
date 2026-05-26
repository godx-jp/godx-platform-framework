package database

import (
	"context"

	"github.com/godx-jp/godx-platform-framework/database/helpers"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TxOptions controls WithTx behaviour.
type TxOptions = helpers.TxOptions

// WithTx runs fn in a transaction with retry on serialization/deadlock errors.
func WithTx(ctx context.Context, pool *pgxpool.Pool, opts TxOptions, fn func(pgx.Tx) error) error {
	return helpers.WithTx(ctx, pool, opts, fn)
}
