package config

import (
	"sync"
	"testing"
	"time"
)

func TestRepositoryTypedAccessors(t *testing.T) {
	r := NewRepository(map[string]any{
		"app": map[string]any{
			"name":  "acme",
			"port":  8080,
			"debug": "yes",
			"rate":  3.14,
			"ttl":   "5s",
			"flags": []any{"a", "b", 3},
			"nested": map[string]any{
				"key": "value",
			},
		},
	})

	if r.GetString("app.name", "fallback") != "acme" {
		t.Fatalf("GetString: want acme")
	}
	if r.GetString("missing", "default") != "default" {
		t.Fatalf("GetString missing: want default")
	}
	if r.GetInt("app.port", -1) != 8080 {
		t.Fatalf("GetInt: want 8080, got %d", r.GetInt("app.port", -1))
	}
	if r.GetInt("app.name", -1) != -1 {
		t.Fatalf("GetInt non-numeric: want default -1")
	}
	if !r.GetBool("app.debug", false) {
		t.Fatalf("GetBool: yes should be true")
	}
	if r.GetFloat("app.rate", 0) != 3.14 {
		t.Fatalf("GetFloat: want 3.14")
	}
	if r.GetDuration("app.ttl", 0) != 5*time.Second {
		t.Fatalf("GetDuration: want 5s")
	}
	if r.Has("missing") {
		t.Fatalf("Has missing key returned true")
	}
	if !r.Has("app.name") {
		t.Fatalf("Has app.name returned false")
	}
	if got := r.GetStringSlice("app.flags", nil); len(got) != 3 || got[0] != "a" || got[2] != "3" {
		t.Fatalf("GetStringSlice: want [a b 3], got %#v", got)
	}
	if m := r.GetMap("app.nested", nil); m["key"] != "value" {
		t.Fatalf("GetMap: want key=value")
	}
}

func TestRepositorySetForgetAllFlat(t *testing.T) {
	r := NewRepository(nil)
	r.Set("db.host", "localhost")
	r.Set("db.port", 5432)
	r.Set("svc.feature.enabled", true)
	if r.GetString("db.host", "") != "localhost" {
		t.Fatalf("Set/Get round trip")
	}
	r.Forget("db.host")
	if r.Has("db.host") {
		t.Fatalf("Forget did not remove key")
	}
	flat := r.AllFlat()
	if flat["db.port"] != 5432 {
		t.Fatalf("AllFlat missing db.port")
	}
	if flat["svc.feature.enabled"] != true {
		t.Fatalf("AllFlat missing svc.feature.enabled")
	}
}

func TestRepositoryConcurrentSafe(t *testing.T) {
	r := NewRepository(map[string]any{"app": map[string]any{"name": "tx"}})
	var wg sync.WaitGroup
	const n = 100
	for i := 0; i < n; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			_ = r.GetString("app.name", "")
			r.Has("app.name")
		}(i)
		go func(i int) {
			defer wg.Done()
			r.Set("counters.x", i)
			r.Forget("counters.y")
		}(i)
	}
	wg.Wait()
}

func TestGetGeneric(t *testing.T) {
	r := NewRepository(map[string]any{
		"s": "hello",
		"i": 42,
		"b": true,
		"d": "1m",
		"f": 1.5,
	})
	if Get[string](r, "s", "x") != "hello" {
		t.Fatalf("Get[string]")
	}
	if Get[int64](r, "i", -1) != 42 {
		t.Fatalf("Get[int64]")
	}
	if Get[int](r, "i", -1) != 42 {
		t.Fatalf("Get[int]")
	}
	if !Get[bool](r, "b", false) {
		t.Fatalf("Get[bool]")
	}
	if Get[time.Duration](r, "d", 0) != time.Minute {
		t.Fatalf("Get[time.Duration]")
	}
	if Get[float64](r, "f", 0) != 1.5 {
		t.Fatalf("Get[float64]")
	}
	if Get[string](r, "missing", "fb") != "fb" {
		t.Fatalf("Get default fallback")
	}
	if Get[string](nil, "any", "fb") != "fb" {
		t.Fatalf("Get nil repo fallback")
	}
}

func TestMerge(t *testing.T) {
	dst := map[string]any{
		"a": 1,
		"nested": map[string]any{
			"x": "old",
			"y": 2,
		},
	}
	src := map[string]any{
		"b": 2,
		"nested": map[string]any{
			"x": "new",
			"z": 3,
		},
	}
	got := Merge(dst, src)
	if got["a"] != 1 || got["b"] != 2 {
		t.Fatalf("Merge top-level")
	}
	n := got["nested"].(map[string]any)
	if n["x"] != "new" || n["y"] != 2 || n["z"] != 3 {
		t.Fatalf("Merge nested: %#v", n)
	}
	// dst untouched.
	if dst["nested"].(map[string]any)["x"] != "old" {
		t.Fatalf("Merge mutated dst")
	}
}
