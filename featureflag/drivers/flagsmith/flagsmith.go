// Package flagsmith is a heavy stub for the Flagsmith SDK.
//
//	import _ "github.com/godx-jp/godx-platform-framework/featureflag/drivers/flagsmith"
package flagsmith

import (
	"context"
	"fmt"
	"sync"

	fdriver "github.com/godx-jp/godx-platform-framework/featureflag/driver"
)

func init() {
	fdriver.Register(fdriver.DriverFlagsmith, func(ctx context.Context, spec fdriver.Spec) (fdriver.Provider, error) {
		if spec.SDKKey == "" {
			return nil, fmt.Errorf("featureflag/flagsmith: spec.SDKKey is required")
		}
		return &provider{sdkKey: spec.SDKKey, endpoint: spec.Endpoint}, nil
	})
}

type provider struct {
	sdkKey   string
	endpoint string
	mu       sync.Mutex
	closed   bool
}

func (p *provider) Name() string { return fdriver.DriverFlagsmith }

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
