// Package introspect registers an OAuth2 token introspection guard stub.
package introspect

import (
	"context"
	"sync"

	adriver "github.com/godx-jp/godx-platform-framework/auth/driver"
)

func init() {
	adriver.Register(adriver.DriverIntrospect, func(_ context.Context, spec adriver.Spec) (adriver.Guard, error) {
		return &guard{url: spec.IntrospectURL}, nil
	})
}

type guard struct {
	url    string
	mu     sync.Mutex
	closed bool
}

func (g *guard) Name() string { return adriver.DriverIntrospect }

func (g *guard) Authenticate(context.Context, *adriver.CredentialRequest) (*adriver.Principal, error) {
	if err := g.checkOpen(); err != nil {
		return nil, err
	}
	return nil, adriver.ErrNotImplemented
}

func (g *guard) Shutdown(context.Context) error {
	g.mu.Lock()
	g.closed = true
	g.mu.Unlock()
	return nil
}

func (g *guard) checkOpen() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed {
		return adriver.ErrClosed
	}
	return nil
}
