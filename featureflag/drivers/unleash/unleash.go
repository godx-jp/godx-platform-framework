// Package unleash is a heavy stub for the Unleash client.
//
//	import _ "github.com/godx-jp/godx-platform-framework/featureflag/drivers/unleash"
package unleash

import (
	"context"
	"fmt"
	"sync"

	fdriver "github.com/godx-jp/godx-platform-framework/featureflag/driver"
)

func init() {
	fdriver.Register(fdriver.DriverUnleash, func(ctx context.Context, spec fdriver.Spec) (fdriver.Provider, error) {
		if spec.Endpoint == "" {
			return nil, fmt.Errorf("featureflag/unleash: spec.Endpoint is required")
		}
		return &provider{endpoint: spec.Endpoint, app: spec.AppName}, nil
	})
}

type provider struct {
	endpoint string
	app      string
	mu       sync.Mutex
	closed   bool
}

func (p *provider) Name() string { return fdriver.DriverUnleash }

func (p *provider) Enabled(_ context.Context, _, _ string, _ map[string]any) (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return false, fdriver.ErrClosed
	}
	return false, fdriver.ErrNotConfigured
}

func (p *provider) Shutdown(_ context.Context) error {
	p.mu.Lock()
	p.closed = true
	p.mu.Unlock()
	return nil
}
