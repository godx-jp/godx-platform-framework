package messaging

import (
	"context"
	"fmt"
	"sync"

	"github.com/godx-jp/godx-platform-framework/framework"
	mdriver "github.com/godx-jp/godx-platform-framework/messaging/driver"
	"github.com/godx-jp/godx-platform-framework/messaging/envelope"
)

const StoreKey = "godx.messaging.manager"

type Manager struct {
	mu      sync.RWMutex
	brokers map[string]mdriver.Broker
	def     string
}

func NewManager() *Manager {
	return &Manager{brokers: map[string]mdriver.Broker{}}
}

func (m *Manager) Add(name string, b mdriver.Broker) error {
	if name == "" || b == nil {
		return fmt.Errorf("messaging: invalid broker")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.brokers[name]; ok {
		return fmt.Errorf("messaging: duplicate connection %q", name)
	}
	m.brokers[name] = b
	if m.def == "" {
		m.def = name
	}
	return nil
}

func (m *Manager) SetDefault(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.brokers[name]; !ok {
		return fmt.Errorf("messaging: unknown connection %q", name)
	}
	m.def = name
	return nil
}

func (m *Manager) broker(name string) (mdriver.Broker, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if name == "" {
		name = m.def
	}
	b, ok := m.brokers[name]
	if !ok {
		return nil, fmt.Errorf("messaging: unknown connection %q", name)
	}
	return b, nil
}

func (m *Manager) Publisher(conn ...string) (*Publisher, error) {
	name := ""
	if len(conn) > 0 {
		name = conn[0]
	}
	b, err := m.broker(name)
	if err != nil {
		return nil, err
	}
	return &Publisher{broker: b}, nil
}

func (m *Manager) Subscriber(conn ...string) (*Subscriber, error) {
	name := ""
	if len(conn) > 0 {
		name = conn[0]
	}
	b, err := m.broker(name)
	if err != nil {
		return nil, err
	}
	return &Subscriber{broker: b}, nil
}

func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.RLock()
	bs := make([]mdriver.Broker, 0, len(m.brokers))
	for _, b := range m.brokers {
		bs = append(bs, b)
	}
	m.mu.RUnlock()
	var first error
	for _, b := range bs {
		if err := b.Close(ctx); err != nil && first == nil {
			first = err
		}
	}
	return first
}

type Publisher struct {
	broker mdriver.Broker
}

func (p *Publisher) Publish(ctx context.Context, e envelope.Event) error {
	if err := e.Validate(); err != nil {
		return err
	}
	body, err := envelope.Encode(e)
	if err != nil {
		return err
	}
	subject := e.Type
	if e.Subject != "" {
		subject = e.Subject
	}
	return p.broker.Publish(ctx, subject, mdriver.Message{
		Body: body,
		Headers: map[string]string{
			"ce-id":   e.ID,
			"ce-type": e.Type,
		},
	})
}

type Subscriber struct {
	broker mdriver.Broker
}

func (s *Subscriber) Subscribe(ctx context.Context, subject string, fn func(context.Context, envelope.Event) error) (mdriver.Subscription, error) {
	return s.broker.Subscribe(ctx, subject, func(ctx context.Context, msg mdriver.Message) error {
		e, err := envelope.Decode(msg.Body)
		if err != nil {
			return err
		}
		return fn(ctx, e)
	})
}

func FromApp(app *framework.App) (*Manager, error) {
	v, ok := app.Lookup(StoreKey)
	if !ok {
		return nil, fmt.Errorf("messaging: Module not initialised")
	}
	mgr, ok := v.(*Manager)
	if !ok {
		return nil, fmt.Errorf("messaging: invalid store %T", v)
	}
	return mgr, nil
}
