package health

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

const (
	PathHealthz = "/healthz"
	PathReadyz  = "/readyz"
)

// Options configures HTTP handlers.
type Options struct {
	// ProbeTimeout bounds each readiness check (default 5s).
	ProbeTimeout time.Duration
}

// Handler returns an http.Handler serving /healthz and /readyz on a stdlib mux.
func Handler(reg *Registry, opts Options) http.Handler {
	if opts.ProbeTimeout <= 0 {
		opts.ProbeTimeout = 5 * time.Second
	}
	mux := http.NewServeMux()
	mux.HandleFunc(PathHealthz, Healthz)
	mux.HandleFunc(PathReadyz, Readyz(reg, opts))
	return mux
}

// Healthz is the liveness handler — returns 200 when the process is up.
func Healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Readyz returns a handler that runs all registered probes.
func Readyz(reg *Registry, opts Options) http.HandlerFunc {
	if opts.ProbeTimeout <= 0 {
		opts.ProbeTimeout = 5 * time.Second
	}
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), opts.ProbeTimeout)
		defer cancel()

		failures := reg.CheckReady(ctx)
		if len(failures) == 0 {
			writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
			return
		}
		body := map[string]any{"status": "not_ready", "failures": failureMessages(failures)}
		writeJSON(w, http.StatusServiceUnavailable, body)
	}
}

func failureMessages(failures map[string]error) map[string]string {
	out := make(map[string]string, len(failures))
	for name, err := range failures {
		if err != nil {
			out[name] = err.Error()
		} else {
			out[name] = "failed"
		}
	}
	return out
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// Mount registers /healthz and /readyz on a chi router (or any http.Handler mux
// via HandleFunc pattern — use Handler for stdlib mux).
func Mount(mux interface {
	HandleFunc(pattern string, handler http.HandlerFunc)
}, reg *Registry, opts Options) {
	if opts.ProbeTimeout <= 0 {
		opts.ProbeTimeout = 5 * time.Second
	}
	mux.HandleFunc(PathHealthz, Healthz)
	mux.HandleFunc(PathReadyz, Readyz(reg, opts))
}
