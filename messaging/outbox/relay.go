package outbox

import (
	"context"
	"time"
)

type Row struct {
	ID         string
	EventID    string
	EventType  string
	Subject    string
	Payload    []byte
	RetryCount int
	CreatedAt  time.Time
}

type Store interface {
	FetchUnpublished(ctx context.Context, limit int) ([]Row, error)
	MarkPublished(ctx context.Context, ids []string) error
}

type RelayOptions struct {
	BatchSize  int
	Source     string
	MaxRetries int
}
