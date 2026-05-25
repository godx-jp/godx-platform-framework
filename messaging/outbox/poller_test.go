package outbox_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/godx-jp/godx-platform-framework/messaging"
	mdriver "github.com/godx-jp/godx-platform-framework/messaging/driver"
	"github.com/godx-jp/godx-platform-framework/messaging/outbox"
)

type memStore struct {
	mu     sync.Mutex
	rows   []outbox.Row
	failed map[string]string
}

func (s *memStore) FetchUnpublished(_ context.Context, limit int) ([]outbox.Row, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.rows) == 0 {
		return nil, nil
	}
	if limit <= 0 || limit > len(s.rows) {
		limit = len(s.rows)
	}
	out := make([]outbox.Row, limit)
	copy(out, s.rows[:limit])
	return out, nil
}

func (s *memStore) MarkPublished(_ context.Context, ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	idSet := map[string]struct{}{}
	for _, id := range ids {
		idSet[id] = struct{}{}
	}
	keep := s.rows[:0]
	for _, row := range s.rows {
		if _, ok := idSet[row.ID]; !ok {
			keep = append(keep, row)
		}
	}
	s.rows = keep
	return nil
}

func (s *memStore) MarkFailed(_ context.Context, id, errMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failed == nil {
		s.failed = map[string]string{}
	}
	s.failed[id] = errMsg
	keep := s.rows[:0]
	for _, row := range s.rows {
		if row.ID != id {
			keep = append(keep, row)
		}
	}
	s.rows = keep
	return nil
}

type stubBroker struct {
	failSubject string
	got         []string
}

func (b *stubBroker) Name() string { return "stub" }

func (b *stubBroker) Publish(_ context.Context, subject string, _ mdriver.Message) error {
	b.got = append(b.got, subject)
	if b.failSubject != "" && subject == b.failSubject {
		return errors.New("publish failed")
	}
	return nil
}

func (b *stubBroker) Subscribe(context.Context, string, mdriver.Handler) (mdriver.Subscription, error) {
	return nil, errors.New("not implemented")
}

func (b *stubBroker) Close(context.Context) error { return nil }

func newPublisher(b mdriver.Broker) *messaging.Publisher {
	mgr := messaging.NewManager()
	_ = mgr.Add("default", b)
	pub, _ := mgr.Publisher()
	return pub
}

func TestRunRelayPartialSuccess(t *testing.T) {
	broker := &stubBroker{failSubject: "bad.event"}
	store := &memStore{rows: []outbox.Row{
		{ID: "1", EventType: "good.event", Payload: []byte(`{}`)},
		{ID: "2", EventType: "bad.event", Payload: []byte(`{}`), RetryCount: 3},
	}}
	pub := newPublisher(broker)
	if err := outbox.RunRelay(context.Background(), store, pub, outbox.RelayOptions{
		Source:     "test://svc",
		MaxRetries: 3,
	}); err != nil {
		t.Fatal(err)
	}
	if len(store.rows) != 0 {
		t.Fatalf("expected all rows cleared, got %+v", store.rows)
	}
	if store.failed["2"] == "" {
		t.Fatal("expected bad row marked failed")
	}
}

func TestRunPollerPublishesUntilEmpty(t *testing.T) {
	broker := &stubBroker{}
	store := &memStore{rows: []outbox.Row{
		{ID: "1", EventType: "orders.placed", Payload: []byte(`{}`)},
	}}
	pub := newPublisher(broker)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go outbox.RunPoller(ctx, store, pub, outbox.PollerOptions{
		RelayOptions: outbox.RelayOptions{Source: "test://svc", BatchSize: 10},
		Interval:     20 * time.Millisecond,
	})
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		store.mu.Lock()
		n := len(store.rows)
		store.mu.Unlock()
		if n == 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("expected all rows published")
}
