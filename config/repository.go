package config

import (
	"strconv"
	"strings"
	"sync"
	"time"
)

// Repository is the typed read/write view over a merged configuration
// tree. Safe for concurrent access. Repository is normally constructed
// by the Manager from a chain of Sources, but it can also be built
// directly via NewRepository for tests and code-driven config.
type Repository struct {
	mu   sync.RWMutex
	data map[string]any
}

// NewRepository returns a Repository wrapping the supplied tree.
// Pass nil for an empty repository. The map is not copied — callers
// that need isolation must clone first.
func NewRepository(data map[string]any) *Repository {
	if data == nil {
		data = map[string]any{}
	}
	return &Repository{data: data}
}

// All returns the underlying tree. The returned map shares storage
// with the repository; callers must not mutate it concurrently.
func (r *Repository) All() map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.data
}

// replace swaps the underlying tree (used by Manager on reload).
func (r *Repository) replace(data map[string]any) {
	if data == nil {
		data = map[string]any{}
	}
	r.mu.Lock()
	r.data = data
	r.mu.Unlock()
}

// Has reports whether key exists. Nested keys are dot-separated.
func (r *Repository) Has(key string) bool {
	_, ok := r.lookup(key)
	return ok
}

// Get returns the value at key. ok==false when the key is missing.
// Returns the raw value — callers normally prefer the typed helpers.
func (r *Repository) Get(key string) (any, bool) {
	return r.lookup(key)
}

// Set writes value at key. Intermediate maps are created as needed.
// Existing non-map intermediates are overwritten.
func (r *Repository) Set(key string, value any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	parts := splitKey(key)
	if len(parts) == 0 {
		return
	}
	m := r.data
	for i := 0; i < len(parts)-1; i++ {
		next, ok := m[parts[i]].(map[string]any)
		if !ok {
			next = map[string]any{}
			m[parts[i]] = next
		}
		m = next
	}
	m[parts[len(parts)-1]] = value
}

// Forget removes key. A missing key is not an error.
func (r *Repository) Forget(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	parts := splitKey(key)
	if len(parts) == 0 {
		return
	}
	m := r.data
	for i := 0; i < len(parts)-1; i++ {
		next, ok := m[parts[i]].(map[string]any)
		if !ok {
			return
		}
		m = next
	}
	delete(m, parts[len(parts)-1])
}

// GetString returns the value at key as a string. Numeric/boolean
// values are stringified Laravel-style; missing keys return def.
func (r *Repository) GetString(key, def string) string {
	v, ok := r.lookup(key)
	if !ok {
		return def
	}
	switch x := v.(type) {
	case string:
		return x
	case bool:
		if x {
			return "true"
		}
		return "false"
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	default:
		return def
	}
}

// GetInt returns the value at key as int64. Strings are parsed,
// floats are truncated, bools become 0/1. Missing or unparseable
// values fall back to def.
func (r *Repository) GetInt(key string, def int64) int64 {
	v, ok := r.lookup(key)
	if !ok {
		return def
	}
	switch x := v.(type) {
	case int:
		return int64(x)
	case int64:
		return x
	case float64:
		return int64(x)
	case bool:
		if x {
			return 1
		}
		return 0
	case string:
		if n, err := strconv.ParseInt(strings.TrimSpace(x), 10, 64); err == nil {
			return n
		}
	}
	return def
}

// GetFloat returns the value at key as float64. Missing or
// unparseable values fall back to def.
func (r *Repository) GetFloat(key string, def float64) float64 {
	v, ok := r.lookup(key)
	if !ok {
		return def
	}
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case string:
		if n, err := strconv.ParseFloat(strings.TrimSpace(x), 64); err == nil {
			return n
		}
	}
	return def
}

// GetBool returns the value at key as bool. Strings are parsed via
// strconv.ParseBool with Laravel-style aliases ("on", "yes", "1").
// Missing values fall back to def.
func (r *Repository) GetBool(key string, def bool) bool {
	v, ok := r.lookup(key)
	if !ok {
		return def
	}
	switch x := v.(type) {
	case bool:
		return x
	case int:
		return x != 0
	case int64:
		return x != 0
	case float64:
		return x != 0
	case string:
		s := strings.ToLower(strings.TrimSpace(x))
		switch s {
		case "1", "true", "yes", "on", "y", "t":
			return true
		case "0", "false", "no", "off", "n", "f", "":
			return false
		}
		if b, err := strconv.ParseBool(s); err == nil {
			return b
		}
	}
	return def
}

// GetDuration returns the value at key as time.Duration. Strings are
// parsed via time.ParseDuration; numeric values are interpreted as
// nanoseconds (matching json.Number conventions). Missing or
// unparseable values fall back to def.
func (r *Repository) GetDuration(key string, def time.Duration) time.Duration {
	v, ok := r.lookup(key)
	if !ok {
		return def
	}
	switch x := v.(type) {
	case time.Duration:
		return x
	case string:
		if d, err := time.ParseDuration(strings.TrimSpace(x)); err == nil {
			return d
		}
	case int:
		return time.Duration(x)
	case int64:
		return time.Duration(x)
	case float64:
		return time.Duration(x)
	}
	return def
}

// GetSlice returns the value at key as []any. Returns def when the
// key is missing or the value is not a slice.
func (r *Repository) GetSlice(key string, def []any) []any {
	v, ok := r.lookup(key)
	if !ok {
		return def
	}
	if s, ok := v.([]any); ok {
		return s
	}
	return def
}

// GetStringSlice returns the value at key as []string, converting
// element types where possible. Returns def when the key is missing
// or the value is not a slice.
func (r *Repository) GetStringSlice(key string, def []string) []string {
	raw := r.GetSlice(key, nil)
	if raw == nil {
		return def
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		switch x := item.(type) {
		case string:
			out = append(out, x)
		case bool:
			if x {
				out = append(out, "true")
			} else {
				out = append(out, "false")
			}
		case int:
			out = append(out, strconv.Itoa(x))
		case int64:
			out = append(out, strconv.FormatInt(x, 10))
		case float64:
			out = append(out, strconv.FormatFloat(x, 'f', -1, 64))
		}
	}
	return out
}

// GetMap returns the value at key as map[string]any. Returns def
// when the key is missing or the value is not a map.
func (r *Repository) GetMap(key string, def map[string]any) map[string]any {
	v, ok := r.lookup(key)
	if !ok {
		return def
	}
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return def
}

// AllFlat returns a flat view of the tree with dot-separated keys.
// Useful for diagnostics and for env-style exporters.
func (r *Repository) AllFlat() map[string]any {
	out := map[string]any{}
	r.mu.RLock()
	defer r.mu.RUnlock()
	flatten("", r.data, out)
	return out
}

func (r *Repository) lookup(key string) (any, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	parts := splitKey(key)
	if len(parts) == 0 {
		return nil, false
	}
	var cur any = r.data
	for _, p := range parts {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = m[p]
		if !ok {
			return nil, false
		}
	}
	return cur, true
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

func flatten(prefix string, in map[string]any, out map[string]any) {
	for k, v := range in {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		if sub, ok := v.(map[string]any); ok {
			flatten(key, sub, out)
			continue
		}
		out[key] = v
	}
}

// Get is a generic helper that returns repo[key] as T or def when
// the key is missing or has the wrong type. Calls Repository methods
// directly for the well-known T values (string, int64, bool,
// time.Duration); falls back to a type assertion otherwise.
func Get[T any](repo *Repository, key string, def T) T {
	if repo == nil {
		return def
	}
	switch any(def).(type) {
	case string:
		v := repo.GetString(key, any(def).(string))
		return any(v).(T)
	case int64:
		v := repo.GetInt(key, any(def).(int64))
		return any(v).(T)
	case int:
		v := repo.GetInt(key, int64(any(def).(int)))
		return any(int(v)).(T)
	case bool:
		v := repo.GetBool(key, any(def).(bool))
		return any(v).(T)
	case float64:
		v := repo.GetFloat(key, any(def).(float64))
		return any(v).(T)
	case time.Duration:
		v := repo.GetDuration(key, any(def).(time.Duration))
		return any(v).(T)
	}
	raw, ok := repo.Get(key)
	if !ok {
		return def
	}
	out, ok := raw.(T)
	if !ok {
		return def
	}
	return out
}

// Merge merges src into dst recursively (last-source-wins). Returns
// a new tree without mutating either input. Map values merge
// element-wise; scalar/slice values are overwritten by src.
func Merge(dst, src map[string]any) map[string]any {
	out := cloneMap(dst)
	for k, v := range src {
		if subSrc, ok := v.(map[string]any); ok {
			if subDst, ok := out[k].(map[string]any); ok {
				out[k] = Merge(subDst, subSrc)
				continue
			}
		}
		out[k] = v
	}
	return out
}

func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		if m, ok := v.(map[string]any); ok {
			out[k] = cloneMap(m)
			continue
		}
		out[k] = v
	}
	return out
}
