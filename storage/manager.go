package storage

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/godx-jp/godx-platform-framework/storage/driver"
)

// Manager owns the lifecycle of every configured Disk and resolves
// disks by name. It is the Go equivalent of Laravel's Storage facade.
//
// Manager is safe for concurrent use by multiple goroutines after
// construction.
type Manager struct {
	mu      sync.RWMutex
	disks   map[string]*Disk
	defKey  string
}

// NewManager builds an empty Manager. Disks are added via AddDisk
// before Init returns. After Init, the manager is read-only except for
// Shutdown.
func NewManager() *Manager {
	return &Manager{disks: map[string]*Disk{}}
}

// AddDisk registers a Disk under name. Returns an error when the disk
// is already registered.
func (m *Manager) AddDisk(name string, d *Disk) error {
	if name == "" {
		return fmt.Errorf("storage: disk name is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, dup := m.disks[name]; dup {
		return fmt.Errorf("storage: disk %q already registered", name)
	}
	m.disks[name] = d
	return nil
}

// SetDefault records which disk Manager.Default returns.
func (m *Manager) SetDefault(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.disks[name]; !ok {
		return fmt.Errorf("storage: cannot set default — disk %q not registered", name)
	}
	m.defKey = name
	return nil
}

// Disk returns the named disk, or (nil, false) when not registered.
func (m *Manager) Disk(name string) (*Disk, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	d, ok := m.disks[name]
	return d, ok
}

// MustDisk is the panic-on-miss variant. Useful in init code where a
// missing disk is a programmer error.
func (m *Manager) MustDisk(name string) *Disk {
	d, ok := m.Disk(name)
	if !ok {
		panic(fmt.Sprintf("storage: disk %q not registered", name))
	}
	return d
}

// Default returns the disk configured as default. Returns (nil, false)
// when SetDefault was never called.
func (m *Manager) Default() (*Disk, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.defKey == "" {
		return nil, false
	}
	d, ok := m.disks[m.defKey]
	return d, ok
}

// DefaultName returns the configured default disk name (may be empty).
func (m *Manager) DefaultName() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.defKey
}

// Disks returns the sorted list of registered disk names.
func (m *Manager) Disks() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := make([]string, 0, len(m.disks))
	for n := range m.disks {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Shutdown disposes every registered driver. Errors are joined into a
// single value so a misbehaving driver does not prevent the others
// from cleaning up.
func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	var errs []error
	for name, d := range m.disks {
		if err := d.driver.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("storage: shutdown disk %q: %w", name, err))
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return joinErrors(errs)
}

// joinErrors avoids pulling in errors.Join for older Go targets while
// remaining trivially compatible.
func joinErrors(errs []error) error {
	if len(errs) == 1 {
		return errs[0]
	}
	msg := "storage: shutdown errors:"
	for _, e := range errs {
		msg += "\n  - " + e.Error()
	}
	return fmt.Errorf("%s", msg)
}

// buildDisk constructs a Disk from a DiskConfig by resolving its
// driver from the global registry. Exported for tests; the module
// uses it internally.
func buildDisk(ctx context.Context, name string, c DiskConfig) (*Disk, error) {
	spec := driver.Spec{
		Name:              c.Driver,
		Disk:              name,
		Root:              c.Root,
		DefaultVisibility: c.DefaultVisibility,
		Bucket:            c.Bucket,
		Region:            c.Region,
		Endpoint:          c.Endpoint,
		UsePathStyle:      c.UsePathStyle,
		AccessKey:         c.AccessKey,
		SecretKey:         c.SecretKey,
		SessionToken:      c.SessionToken,
		PublicURL:         c.PublicURL,
		Extra:             c.Extra,
	}
	drv, err := driver.New(ctx, spec)
	if err != nil {
		return nil, err
	}
	return &Disk{name: name, driver: drv, config: c}, nil
}
