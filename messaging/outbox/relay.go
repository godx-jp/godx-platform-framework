package outbox

import (
	"context"
	"time"

	"github.com/godx-jp/godx-platform-framework/messaging"
	"github.com/godx-jp/godx-platform-framework/messaging/envelope"
)

type Row struct {
	ID        string
	EventType string
	Payload   []byte
	CreatedAt time.Time
}

type Store interface {
	FetchUnpublished(ctx context.Context, limit int) ([]Row, error)
	MarkPublished(ctx context.Context, ids []string) error
}

type RelayOptions struct {
	BatchSize int
	Source    string
}

func RunRelay(ctx context.Context, store Store, pub *messaging.Publisher, opts RelayOptions) error {
	if store == nil || pub == nil {
		return nil
	}
	limit := opts.BatchSize
	if limit <= 0 {
		limit = 100
	}
	rows, err := store.FetchUnpublished(ctx, limit)
	if err != nil {
		return err
	}
	var ids []string
	for _, row := range rows {
		if err := pub.Publish(ctx, envelope.Event{
			ID:     row.ID,
			Source: opts.Source,
			Type:   row.EventType,
			Data:   row.Payload,
		}); err != nil {
			return err
		}
		ids = append(ids, row.ID)
	}
	if len(ids) == 0 {
		return nil
	}
	return store.MarkPublished(ctx, ids)
}
