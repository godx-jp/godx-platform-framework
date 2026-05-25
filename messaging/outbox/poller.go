package outbox

import (
	"context"
	"time"

	"github.com/godx-jp/godx-platform-framework/messaging"
	"github.com/godx-jp/godx-platform-framework/messaging/envelope"
)

// RetryStore extends Store with permanent-failure marking after max retries.
type RetryStore interface {
	Store
	MarkFailed(ctx context.Context, id string, errMsg string) error
}

// PollerOptions configures the background outbox relay loop.
type PollerOptions struct {
	RelayOptions
	Interval   time.Duration
	MaxRetries int
	OnError    func(error)
}

// RunPoller relays unpublished outbox rows until ctx is cancelled.
func RunPoller(ctx context.Context, store Store, pub *messaging.Publisher, opts PollerOptions) {
	interval := opts.Interval
	if interval <= 0 {
		interval = 2 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := relayBatch(ctx, store, pub, opts.RelayOptions); err != nil && opts.OnError != nil {
				opts.OnError(err)
			}
		}
	}
}

func relayBatch(ctx context.Context, store Store, pub *messaging.Publisher, opts RelayOptions) error {
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
	var retryStore RetryStore
	if rs, ok := store.(RetryStore); ok {
		retryStore = rs
	}
	var published []string
	for _, row := range rows {
		eventID := row.EventID
		if eventID == "" {
			eventID = row.ID
		}
		subject := row.Subject
		if subject == "" {
			subject = row.EventType
		}
		err := pub.Publish(ctx, envelope.Event{
			ID:      eventID,
			Source:  opts.Source,
			Type:    row.EventType,
			Subject: subject,
			Data:    row.Payload,
		})
		if err != nil {
			if retryStore != nil && opts.MaxRetries > 0 && row.RetryCount >= opts.MaxRetries {
				_ = retryStore.MarkFailed(ctx, row.ID, err.Error())
			}
			continue
		}
		published = append(published, row.ID)
	}
	if len(published) == 0 {
		return nil
	}
	return store.MarkPublished(ctx, published)
}

func RunRelay(ctx context.Context, store Store, pub *messaging.Publisher, opts RelayOptions) error {
	return relayBatch(ctx, store, pub, opts)
}
