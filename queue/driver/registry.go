package driver

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

const (
	DriverMemory = "memory"
	DriverRedis  = "redis"
	DriverSQS    = "sqs"
	DriverKafka  = "kafka"
	DriverNATS   = "nats"
)

var (
	regMu sync.RWMutex
	reg   = map[string]Constructor{}
)

func Register(name string, c Constructor) {
	if name == "" {
		panic("queue: Register called with empty driver name")
	}
	if c == nil {
		panic("queue: Register called with nil Constructor for " + name)
	}
	regMu.Lock()
	defer regMu.Unlock()
	reg[name] = c
}

func Lookup(name string) Constructor {
	regMu.RLock()
	defer regMu.RUnlock()
	return reg[name]
}

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

func New(ctx context.Context, spec Spec) (Backend, error) {
	if spec.Name == "" {
		return nil, fmt.Errorf("queue: driver name is required")
	}
	c := Lookup(spec.Name)
	if c == nil {
		return nil, fmt.Errorf(
			"queue: driver %q not registered — did you forget to import "+
				"github.com/godx-jp/godx-platform-framework/queue/drivers/%s ?",
			spec.Name, spec.Name)
	}
	return c(ctx, spec)
}
