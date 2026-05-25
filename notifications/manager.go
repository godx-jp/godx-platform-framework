package notifications

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/godx-jp/godx-platform-framework/events"
	ndriver "github.com/godx-jp/godx-platform-framework/notifications/driver"
)

const (
	EventSending = "notification.sending"
	EventSent    = "notification.sent"
	EventFailed  = "notification.failed"
)

// Manager owns named notification channels.
type Manager struct {
	mu       sync.RWMutex
	channels map[string]ndriver.Channel
	def      string
	bus      events.Bus
}

// NewManager returns an empty Manager.
func NewManager() *Manager {
	return &Manager{channels: map[string]ndriver.Channel{}}
}

// SetBus attaches an optional events Bus.
func (m *Manager) SetBus(bus events.Bus) {
	m.mu.Lock()
	m.bus = bus
	m.mu.Unlock()
}

// AddChannel registers ch under name.
func (m *Manager) AddChannel(name string, ch ndriver.Channel) error {
	if name == "" || ch == nil {
		return fmt.Errorf("notifications: AddChannel: name and channel required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.channels[name]; exists {
		return fmt.Errorf("notifications: channel %q already registered", name)
	}
	m.channels[name] = ch
	if m.def == "" {
		m.def = name
	}
	return nil
}

// SetDefault flags name as the default channel (used when Via returns
// driver names that match registered channel keys).
func (m *Manager) SetDefault(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.channels[name]; !ok {
		return fmt.Errorf("notifications: SetDefault(%q): not registered", name)
	}
	m.def = name
	return nil
}

// Channel returns a registered channel by name.
func (m *Manager) Channel(name string) (ndriver.Channel, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ch, ok := m.channels[name]
	if !ok {
		return nil, fmt.Errorf("notifications: channel %q not registered", name)
	}
	return ch, nil
}

// Names returns sorted registered channel names.
func (m *Manager) Names() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]string, 0, len(m.channels))
	for n := range m.channels {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Notifier returns a sender bound to this manager.
func (m *Manager) Notifier() *Notifier {
	m.mu.RLock()
	bus := m.bus
	m.mu.RUnlock()
	return &Notifier{mgr: m, bus: bus}
}

// Send delivers notification to every channel returned by Via.
func (m *Manager) Send(ctx context.Context, notifiable Notifiable, notification Notification) error {
	return m.Notifier().Send(ctx, notifiable, notification)
}

// Shutdown shuts every registered channel down.
func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	channels := make([]ndriver.Channel, 0, len(m.channels))
	for _, ch := range m.channels {
		channels = append(channels, ch)
	}
	m.mu.Unlock()
	var errs []error
	for _, ch := range channels {
		if err := ch.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (m *Manager) dispatch(ctx context.Context, name string, payload map[string]any) {
	bus := m.bus
	if bus == nil {
		if b, ok := events.FromContext(ctx); ok {
			bus = b
		}
	}
	if bus == nil {
		return
	}
	_ = bus.Dispatch(ctx, events.Event{Name: name, Payload: payload})
}

// Notifier sends notifications through a Manager.
type Notifier struct {
	mgr *Manager
	bus events.Bus
}

// Send delivers notification to each channel from Via.
func (n *Notifier) Send(ctx context.Context, notifiable Notifiable, notification Notification) error {
	if n.mgr == nil {
		return fmt.Errorf("notifications: no manager configured")
	}
	channels := notification.Via(notifiable)
	if len(channels) == 0 {
		return fmt.Errorf("notifications: notification declares no channels")
	}
	var errs []error
	for _, chName := range channels {
		ch, err := n.mgr.Channel(chName)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		payload := map[string]any{"channel": chName, "driver": ch.Name()}
		n.mgr.dispatch(ctx, EventSending, payload)
		if err := ch.Send(ctx, notifiable, notification); err != nil {
			payload["error"] = err.Error()
			n.mgr.dispatch(ctx, EventFailed, payload)
			errs = append(errs, fmt.Errorf("notifications: %s: %w", chName, err))
			continue
		}
		n.mgr.dispatch(ctx, EventSent, payload)
	}
	return errors.Join(errs...)
}
