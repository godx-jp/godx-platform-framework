package mail

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/godx-jp/godx-platform-framework/events"
	mdriver "github.com/godx-jp/godx-platform-framework/mail/driver"
)

const (
	EventSending = "mail.sending"
	EventSent    = "mail.sent"
	EventFailed  = "mail.failed"
)

// Manager owns named mail transports and an optional events Bus.
type Manager struct {
	mu         sync.RWMutex
	transports map[string]mdriver.Transport
	def        string
	from       string
	bus        events.Bus
}

// NewManager returns an empty Manager.
func NewManager() *Manager {
	return &Manager{transports: map[string]mdriver.Transport{}}
}

// SetBus attaches an events Bus used when Mailer.Send dispatches
// lifecycle events. Optional — Send works without a bus.
func (m *Manager) SetBus(bus events.Bus) {
	m.mu.Lock()
	m.bus = bus
	m.mu.Unlock()
}

// SetDefaultFrom sets the fallback From address for all mailers.
func (m *Manager) SetDefaultFrom(from string) {
	m.mu.Lock()
	m.from = from
	m.mu.Unlock()
}

// AddTransport registers t under name.
func (m *Manager) AddTransport(name string, t mdriver.Transport) error {
	if name == "" || t == nil {
		return fmt.Errorf("mail: AddTransport: name and transport required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.transports[name]; exists {
		return fmt.Errorf("mail: transport %q already registered", name)
	}
	m.transports[name] = t
	if m.def == "" {
		m.def = name
	}
	return nil
}

// SetDefault flags name as the default transport.
func (m *Manager) SetDefault(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.transports[name]; !ok {
		return fmt.Errorf("mail: SetDefault(%q): not registered", name)
	}
	m.def = name
	return nil
}

// DefaultTransport returns the default transport, or nil.
func (m *Manager) DefaultTransport() mdriver.Transport {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.transports[m.def]
}

// Transport returns the named transport.
func (m *Manager) Transport(name string) (mdriver.Transport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.transports[name]
	if !ok {
		return nil, fmt.Errorf("mail: transport %q not registered", name)
	}
	return t, nil
}

// Names returns sorted registered transport names.
func (m *Manager) Names() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]string, 0, len(m.transports))
	for n := range m.transports {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Mailer returns a fluent builder bound to the default transport.
// Pass a name to target a specific transport.
func (m *Manager) Mailer(name ...string) (*Mailer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	key := m.def
	if len(name) > 0 && name[0] != "" {
		key = name[0]
	}
	t, ok := m.transports[key]
	if !ok {
		return nil, fmt.Errorf("mail: transport %q not registered", key)
	}
	return &Mailer{
		mgr:       m,
		transport: t,
		bus:       m.bus,
		from:      m.from,
	}, nil
}

// Shutdown shuts every registered transport down.
func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	transports := make([]mdriver.Transport, 0, len(m.transports))
	for _, t := range m.transports {
		transports = append(transports, t)
	}
	m.mu.Unlock()
	var errs []error
	for _, t := range transports {
		if err := t.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// dispatch emits an event when a bus is available on the manager or
// attached to ctx.
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
