// Package openfeature is a heavy stub for the OpenFeature SDK.
//
//	import _ "github.com/godx-jp/godx-platform-framework/featureflag/drivers/openfeature"
package openfeature

import (
	"context"
	"fmt"
	"sync"

	fdriver "github.com/godx-jp/godx-platform-framework/featureflag/driver"
)

func init() {
	fdriver.Register(fdriver.DriverOpenFeature, func(ctx context.Context, spec fdriver.Spec) (fdriver.Provider, error) {
		if spec.Endpoint == "" {
			return nil, fmt.Errorf("featureflag/openfeature: spec.Endpoint is required")
		}
		return &provider{endpoint: spec.Endpoint}, nil
	})
}

type provider struct {
	endpoint string
	mu       sync.Mutex
	closed   bool
}

func (p *provider) Name() string { return fdriver.DriverOpenFeature }

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
