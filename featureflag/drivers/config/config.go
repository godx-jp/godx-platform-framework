package configdriver

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/godx-jp/godx-platform-framework/config"
	fdriver "github.com/godx-jp/godx-platform-framework/featureflag/driver"
)

func init() {
	fdriver.Register(fdriver.DriverConfig, func(ctx context.Context, spec fdriver.Spec) (fdriver.Provider, error) {
		if spec.Repo == nil {
			return nil, fmt.Errorf("featureflag/config: Repository is required")
		}
		prefix := spec.Prefix
		if prefix == "" {
			prefix = "flags"
		}
		return &provider{repo: spec.Repo, prefix: prefix}, nil
	})
}

type provider struct {
	repo   *config.Repository
	prefix string

	mu     sync.Mutex
	closed bool
}

func (p *provider) Name() string { return fdriver.DriverConfig }

func (p *provider) Enabled(_ context.Context, flag, user string, _ map[string]any) (bool, error) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return false, fdriver.ErrClosed
	}
	p.mu.Unlock()

	key := p.prefix + "." + flag
	if p.repo.GetBool(key, false) {
		return true, nil
	}
	usersKey := key + ".users"
	if !p.repo.Has(usersKey) {
		return false, nil
	}
	slice := p.repo.GetStringSlice(usersKey, nil)
	if len(slice) == 0 {
		raw := p.repo.GetString(usersKey, "")
		if raw == "" {
			return false, nil
		}
		for _, u := range strings.Split(raw, ",") {
			if strings.TrimSpace(u) == user {
				return true, nil
			}
		}
		return false, nil
	}
	for _, u := range slice {
		if u == user {
			return true, nil
		}
	}
	return false, nil
}

func (p *provider) Shutdown(_ context.Context) error {
	p.mu.Lock()
	p.closed = true
	p.mu.Unlock()
	return nil
}
