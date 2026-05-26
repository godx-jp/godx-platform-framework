package database

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"sort"
	"sync"
	"sync/atomic"
)

// Manager holds named database connections.
type Manager struct {
	mu sync.RWMutex

	cfg          Config
	defaultName  string
	writeName    string
	readNames    []string
	readStrategy ReadStrategy
	sticky       bool
	readIndex    atomic.Uint64
	connections  map[string]*Connection
	metricsStop  context.CancelFunc
}

// NewManager returns an empty Manager.
func NewManager() *Manager {
	return &Manager{connections: map[string]*Connection{}}
}

func (m *Manager) configureRouting(cfg Config) {
	m.cfg = cfg
	m.defaultName = cfg.DefaultConnection
	m.writeName = cfg.WriteConnection
	m.readNames = append([]string(nil), cfg.ReadConnections...)
	m.readStrategy = cfg.ReadStrategy
	m.sticky = cfg.Sticky
}

// AddConnection registers conn under its name.
func (m *Manager) AddConnection(conn *Connection) error {
	if conn == nil {
		return errors.New("database: AddConnection called with nil connection")
	}
	if conn.Name() == "" {
		return errors.New("database: AddConnection called with empty name")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, dup := m.connections[conn.Name()]; dup {
		return fmt.Errorf("database: connection %q already registered", conn.Name())
	}
	m.connections[conn.Name()] = conn
	return nil
}

// SetDefault marks name as Default().
func (m *Manager) SetDefault(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.connections[name]; !ok {
		return fmt.Errorf("database: default connection %q is not registered", name)
	}
	m.defaultName = name
	return nil
}

// Connection returns a named connection.
func (m *Manager) Connection(name string) (*Connection, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.connections[name]
	if !ok {
		return nil, fmt.Errorf("database: connection %q not registered", name)
	}
	return c, nil
}

// Default returns the default connection.
func (m *Manager) Default() (*Connection, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.defaultName == "" {
		return nil, errors.New("database: Default called before SetDefault")
	}
	c, ok := m.connections[m.defaultName]
	if !ok {
		return nil, fmt.Errorf("database: default connection %q not registered", m.defaultName)
	}
	return c, nil
}

// Write returns the configured write (primary) connection.
func (m *Manager) Write() (*Connection, error) {
	name := m.writeName
	if name == "" {
		return m.Default()
	}
	return m.Connection(name)
}

// Read returns a read connection or the write connection when none configured.
// When sticky is enabled and ctx was MarkWritten, returns write.
func (m *Manager) Read(ctx context.Context) (*Connection, error) {
	if m.sticky && WasWritten(ctx) {
		return m.Write()
	}
	if len(m.readNames) == 0 {
		return m.Write()
	}
	name := m.pickRead()
	return m.Connection(name)
}

func (m *Manager) pickRead() string {
	switch m.readStrategy {
	case ReadRandom:
		return m.readNames[rand.IntN(len(m.readNames))]
	default:
		i := m.readIndex.Add(1)
		return m.readNames[int(i)%len(m.readNames)]
	}
}

// Connections returns sorted registered connection names.
func (m *Manager) Connections() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]string, 0, len(m.connections))
	for name := range m.connections {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// Config returns a copy of module routing config.
func (m *Manager) Config() Config { return m.cfg }

func (m *Manager) setMetricsStop(cancel context.CancelFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.metricsStop = cancel
}

// Shutdown closes all connections and stops metrics collection.
func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	stop := m.metricsStop
	m.metricsStop = nil
	conns := m.connections
	m.connections = map[string]*Connection{}
	m.defaultName = ""
	m.mu.Unlock()

	if stop != nil {
		stop()
	}

	var errs []error
	for name, c := range conns {
		if err := c.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("database: shutdown %q: %w", name, err))
		}
	}
	return errors.Join(errs...)
}
