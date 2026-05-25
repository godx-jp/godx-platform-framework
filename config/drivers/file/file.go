// Package file is the on-disk file Source. Loads YAML, JSON, or TOML
// from a path on every Load and supports change-watching via mtime
// polling — opt in by adding a Watcher loop. The mtime poll deliberately
// avoids fsnotify so the file driver stays dependency-light.
package file

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"

	cdriver "github.com/godx-jp/godx-platform-framework/config/driver"
)

func init() {
	cdriver.Register(cdriver.DriverFile, func(_ context.Context, spec cdriver.Spec) (cdriver.Source, error) {
		return New(spec.Path, spec.Format, spec.Optional)
	})
}

// New constructs a file Source. format may be "yaml", "json", or
// "toml"; pass "" to infer from the path extension. When optional
// is true, a missing path becomes an empty tree on Load instead of
// an error.
func New(path, format string, optional bool) (cdriver.Source, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("config/file: path is required")
	}
	if format == "" {
		format = inferFormat(path)
	}
	if format == "" {
		return nil, cdriver.ErrUnsupportedFormat
	}
	switch format {
	case "yaml", "json", "toml":
	default:
		return nil, fmt.Errorf("%w: %q", cdriver.ErrUnsupportedFormat, format)
	}
	return &source{
		path:     path,
		format:   format,
		optional: optional,
	}, nil
}

type source struct {
	path     string
	format   string
	optional bool

	mu        sync.Mutex
	closed    bool
	watchOnce sync.Once
	watchStop chan struct{}
}

func (s *source) Name() string { return cdriver.DriverFile }

func (s *source) Load(ctx context.Context) (map[string]any, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, cdriver.ErrClosed
	}
	s.mu.Unlock()

	raw, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if s.optional {
				return map[string]any{}, nil
			}
			return nil, fmt.Errorf("%w: %s", cdriver.ErrFileMissing, s.path)
		}
		return nil, fmt.Errorf("config/file: read %s: %w", s.path, err)
	}
	out, err := decode(s.format, raw)
	if err != nil {
		return nil, fmt.Errorf("config/file: decode %s as %s: %w", s.path, s.format, err)
	}
	return normalise(out), nil
}

// Watch polls the file's mtime every second and invokes onChange
// when it changes. Implements cdriver.Watcher.
func (s *source) Watch(ctx context.Context, onChange func()) error {
	if onChange == nil {
		return nil
	}
	var startErr error
	s.watchOnce.Do(func() {
		s.watchStop = make(chan struct{})
		info, err := os.Stat(s.path)
		if err != nil && !s.optional {
			startErr = fmt.Errorf("config/file: stat %s for watch: %w", s.path, err)
			return
		}
		var lastMod time.Time
		if info != nil {
			lastMod = info.ModTime()
		}
		go func() {
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-s.watchStop:
					return
				case <-ticker.C:
					info, err := os.Stat(s.path)
					if err != nil {
						continue
					}
					if !info.ModTime().Equal(lastMod) {
						lastMod = info.ModTime()
						onChange()
					}
				}
			}
		}()
	})
	return startErr
}

func (s *source) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	if s.watchStop != nil {
		close(s.watchStop)
	}
	return nil
}

func inferFormat(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		return "yaml"
	case ".json":
		return "json"
	case ".toml":
		return "toml"
	}
	return ""
}

func decode(format string, raw []byte) (map[string]any, error) {
	switch format {
	case "yaml":
		var out map[string]any
		if err := yaml.Unmarshal(raw, &out); err != nil {
			return nil, err
		}
		return out, nil
	case "json":
		var out map[string]any
		if len(raw) == 0 {
			return map[string]any{}, nil
		}
		if err := json.Unmarshal(raw, &out); err != nil {
			return nil, err
		}
		return out, nil
	case "toml":
		var out map[string]any
		if err := toml.Unmarshal(raw, &out); err != nil {
			return nil, err
		}
		return out, nil
	}
	return nil, cdriver.ErrUnsupportedFormat
}

// normalise turns map[any]any and map[string]interface{} (yaml.v2 / yaml.v3
// edge cases for unkeyed maps) and []interface{} into the canonical
// map[string]any / []any shape the Repository expects.
func normalise(v any) map[string]any {
	if v == nil {
		return map[string]any{}
	}
	out, _ := convert(v).(map[string]any)
	if out == nil {
		out = map[string]any{}
	}
	return out
}

func convert(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, vv := range x {
			out[k] = convert(vv)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(x))
		for k, vv := range x {
			out[fmt.Sprint(k)] = convert(vv)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = convert(item)
		}
		return out
	}
	return v
}
