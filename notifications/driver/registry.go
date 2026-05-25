package driver

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// Canonical channel driver names.
const (
	DriverLog      = "log"
	DriverMail     = "mail"
	DriverSlack    = "slack"
	DriverDiscord  = "discord"
	DriverWebhook  = "webhook"
	DriverDatabase = "database"
)

var (
	regMu sync.RWMutex
	reg   = map[string]Constructor{}
)

// Register adds a Constructor under name.
func Register(name string, c Constructor) {
	if name == "" {
		panic("notifications: Register called with empty driver name")
	}
	if c == nil {
		panic("notifications: Register called with nil Constructor for " + name)
	}
	regMu.Lock()
	defer regMu.Unlock()
	reg[name] = c
}

// Lookup returns the registered Constructor for name, or nil.
func Lookup(name string) Constructor {
	regMu.RLock()
	defer regMu.RUnlock()
	return reg[name]
}

// Names returns the sorted list of registered driver names.
func Names() []string {
	regMu.RLock()
	defer regMu.RUnlock()
	out := make([]string, 0, len(reg))
	for n := range reg {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// New constructs a Channel from spec by looking up spec.Name.
func New(ctx context.Context, spec Spec) (Channel, error) {
	if spec.Name == "" {
		return nil, fmt.Errorf("notifications: driver name is required")
	}
	c := Lookup(spec.Name)
	if c == nil {
		return nil, fmt.Errorf(
			"notifications: driver %q not registered — did you forget to import "+
				"github.com/godx-jp/godx-platform-framework/notifications/drivers/%s ?",
			spec.Name, spec.Name)
	}
	return c(ctx, spec)
}
