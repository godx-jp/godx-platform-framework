package helpers

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TxOptions controls WithTx behaviour.
type TxOptions struct {
	IsoLevel     pgx.TxIsoLevel
	MaxRetries   int
	RetryBackoff time.Duration
}

// WithTx runs fn inside a transaction with auto-retry on serialization/deadlock.
func WithTx(ctx context.Context, pool *pgxpool.Pool, opts TxOptions, fn func(pgx.Tx) error) error {
	if pool == nil {
		return errors.New("database: pool is required")
	}
	if opts.MaxRetries == 0 {
		opts.MaxRetries = 3
	}
	if opts.RetryBackoff == 0 {
		opts.RetryBackoff = 50 * time.Millisecond
	}
	if opts.IsoLevel == "" {
		opts.IsoLevel = pgx.ReadCommitted
	}

	var lastErr error
	for attempt := 0; attempt <= opts.MaxRetries; attempt++ {
		err := runTx(ctx, pool, opts.IsoLevel, fn)
		if err == nil {
			return nil
		}
		lastErr = err
		if !isRetryableTxError(err) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(opts.RetryBackoff * time.Duration(attempt+1)):
		}
	}
	return fmt.Errorf("database: tx exhausted retries: %w", lastErr)
}

func runTx(ctx context.Context, pool *pgxpool.Pool, iso pgx.TxIsoLevel, fn func(pgx.Tx) error) error {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: iso})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func isRetryableTxError(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	switch pgErr.Code {
	case "40001", "40P01":
		return true
	}
	return false
}
