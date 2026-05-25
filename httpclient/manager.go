package httpclient

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"sync"
)

// Manager owns named HTTP clients.
type Manager struct {
	mu      sync.RWMutex
	clients map[string]*Client
	def     string
}

func NewManager() *Manager {
	return &Manager{clients: map[string]*Client{}}
}

func (m *Manager) AddClient(name string, c *Client) error {
	if name == "" || c == nil {
		return fmt.Errorf("httpclient: AddClient: name and client required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.clients[name]; exists {
		return fmt.Errorf("httpclient: client %q already registered", name)
	}
	m.clients[name] = c
	if m.def == "" {
		m.def = name
	}
	return nil
}

func (m *Manager) SetDefault(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.clients[name]; !ok {
		return fmt.Errorf("httpclient: SetDefault(%q): not registered", name)
	}
	m.def = name
	return nil
}

func (m *Manager) Default() *Client {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.clients[m.def]
}

func (m *Manager) Client(name string) (*Client, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.clients[name]
	if !ok {
		return nil, fmt.Errorf("httpclient: client %q not registered", name)
	}
	return c, nil
}

func (m *Manager) Names() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]string, 0, len(m.clients))
	for n := range m.clients {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Get issues GET on the default client.
func (m *Manager) Get(ctx context.Context, path string) (*http.Response, error) {
	d := m.Default()
	if d == nil {
		return nil, fmt.Errorf("httpclient: no default client")
	}
	return d.Get(ctx, path)
}

func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	clients := make([]*Client, 0, len(m.clients))
	for _, c := range m.clients {
		clients = append(clients, c)
	}
	m.mu.Unlock()
	var errs []error
	for _, c := range clients {
		if err := c.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
