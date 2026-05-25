package driver

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// Canonical driver names. Constants make typos impossible and let the
// storage package reference drivers without importing each
// implementation package (which would defeat the opt-in heavy-driver
// pattern).
const (
	DriverLocal  = "local"
	DriverMemory = "memory"
	DriverS3     = "s3"
	DriverGCS    = "gcs"
	DriverAzure  = "azure"
	DriverMinIO  = "minio"
)

var (
	registryMu sync.RWMutex
	registry   = map[string]Constructor{}
)

// Register associates name with constructor in the global driver
// registry. Each driver package calls Register from its init function.
// Re-registering an existing name panics — drivers should pick unique
// names.
func Register(name string, constructor Constructor) {
	if name == "" {
		panic("storage/driver: Register called with empty name")
	}
	if constructor == nil {
		panic(fmt.Sprintf("storage/driver: Register called with nil constructor for %q", name))
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, dup := registry[name]; dup {
		panic(fmt.Sprintf("storage/driver: driver %q already registered", name))
	}
	registry[name] = constructor
}

// Lookup returns the constructor previously registered for name, or
// (nil, false) if no such driver exists.
func Lookup(name string) (Constructor, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	c, ok := registry[name]
	return c, ok
}

// Names returns the sorted list of currently-registered driver names.
// Useful for emitting helpful error messages.
func Names() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// New looks up the driver named in s.Name and invokes its constructor.
// Returns a wrapped error when the driver is not registered, including
// the list of known drivers and a hint about the blank-import pattern
// required for heavy drivers.
func New(ctx context.Context, s Spec) (Driver, error) {
	c, ok := Lookup(s.Name)
	if !ok {
		return nil, fmt.Errorf(
			"storage: unknown driver %q (known: %v). Heavy drivers (s3, gcs, azure, minio) "+
				"require a blank import such as `_ \"github.com/godx-jp/godx-platform-framework/storage/drivers/s3\"`",
			s.Name, Names(),
		)
	}
	d, err := c(ctx, s)
	if err != nil {
		return nil, fmt.Errorf("storage: driver %q: %w", s.Name, err)
	}
	return d, nil
}
