// Package env is the OS-environment Source. Variables matching the
// configured prefix are stripped of the prefix and split on "__"
// into nested keys (Laravel: PHP_INI__SCRIPT becomes
// php_ini.script). Single underscores are preserved inside one
// segment so APP_NAME stays at app_name, not app.name — set the
// nested separator with WithSeparator on a manual constructor if
// the default conflicts with your scheme.
package env

import (
	"context"
	"os"
	"strings"
	"sync"

	cdriver "github.com/godx-jp/godx-platform-framework/config/driver"
)

func init() {
	cdriver.Register(cdriver.DriverEnv, func(_ context.Context, spec cdriver.Spec) (cdriver.Source, error) {
		return &source{prefix: spec.Prefix, separator: "__"}, nil
	})
}

type source struct {
	prefix    string
	separator string
	mu        sync.Mutex
	closed    bool
}

// New constructs an env Source without going through the registry —
// useful for tests and code-driven setup that need to override the
// nested-key separator. Use cdriver.Lookup("env") in production.
func New(prefix, separator string) cdriver.Source {
	if separator == "" {
		separator = "__"
	}
	return &source{prefix: prefix, separator: separator}
}

func (s *source) Name() string { return cdriver.DriverEnv }

func (s *source) Load(ctx context.Context) (map[string]any, error) {
	s.mu.Lock()
	closed := s.closed
	s.mu.Unlock()
	if closed {
		return nil, cdriver.ErrClosed
	}
	out := map[string]any{}
	for _, kv := range os.Environ() {
		i := strings.IndexByte(kv, '=')
		if i < 1 {
			continue
		}
		key, val := kv[:i], kv[i+1:]
		if s.prefix != "" {
			if !strings.HasPrefix(key, s.prefix) {
				continue
			}
			key = strings.TrimPrefix(key, s.prefix)
		}
		key = strings.ToLower(key)
		key = strings.ReplaceAll(key, s.separator, ".")
		setNested(out, key, val)
	}
	return out, nil
}

func (s *source) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	return nil
}

func setNested(root map[string]any, key string, val any) {
	parts := splitKey(key)
	if len(parts) == 0 {
		return
	}
	m := root
	for i := 0; i < len(parts)-1; i++ {
		next, ok := m[parts[i]].(map[string]any)
		if !ok {
			next = map[string]any{}
			m[parts[i]] = next
		}
		m = next
	}
	m[parts[len(parts)-1]] = val
}

func splitKey(key string) []string {
	if key == "" {
		return nil
	}
	parts := strings.Split(key, ".")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}
