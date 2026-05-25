// Package framework is the lightweight backbone of godx-platform-framework.
//
// An [App] composes [Module]s. Modules are initialised in the order they were
// registered; on shutdown (SIGINT/SIGTERM or context cancel) they are torn down
// in reverse order. Modules can share state through [App.Store] / [App.Lookup].
//
//	app := framework.New("my-service", "1.0.0").
//		Use(observability.Module)
//	if err := app.Run(ctx); err != nil {
//		log.Fatal(err)
//	}
package framework

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// Module is the unit of pluggable behaviour. Implementations should be
// stateless package-level vars where possible; per-app state lives in [App].
type Module interface {
	// Name returns a unique identifier used for logging and Lookup keys.
	Name() string
	// Init wires the module into the app. Implementations may register
	// shutdown hooks via app.OnShutdown and stash shared state via app.Store.
	Init(ctx context.Context, app *App) error
}

// ShutdownFunc tears down resources owned by a module. Called in reverse
// registration order during [App.Shutdown].
type ShutdownFunc func(ctx context.Context) error

// App is the running instance of a service backed by godx-platform-framework.
type App struct {
	name    string
	version string

	mu        sync.RWMutex
	modules   []Module
	store     map[string]any
	shutdowns []ShutdownFunc

	initOnce sync.Once
	initErr  error
}

// New returns a new App with the given service identity. Both fields surface
// in telemetry (`service.name`, `service.version`).
func New(name, version string) *App {
	return &App{
		name:    name,
		version: version,
		store:   make(map[string]any),
	}
}

// Name reports the service name passed to [New].
func (a *App) Name() string { return a.name }

// Version reports the service version passed to [New].
func (a *App) Version() string { return a.version }

// Use registers a module. Modules are initialised in registration order.
// Calling Use after Init is a no-op and returns the App unchanged.
func (a *App) Use(m Module) *App {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.modules = append(a.modules, m)
	return a
}

// Store sets a shared value addressable by [App.Lookup]. Typical key:
// "<module>.<resource>" e.g. "observability.provider".
func (a *App) Store(key string, value any) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.store[key] = value
}

// Lookup retrieves a value previously set by [App.Store].
func (a *App) Lookup(key string) (any, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	v, ok := a.store[key]
	return v, ok
}

// OnShutdown appends a teardown hook. Hooks fire in reverse registration order.
func (a *App) OnShutdown(fn ShutdownFunc) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.shutdowns = append(a.shutdowns, fn)
}

// Init runs all module Init functions in registration order. Safe to call
// multiple times; subsequent calls reuse the first result.
func (a *App) Init(ctx context.Context) error {
	a.initOnce.Do(func() {
		for _, m := range a.modules {
			if err := m.Init(ctx, a); err != nil {
				a.initErr = fmt.Errorf("framework: init module %q: %w", m.Name(), err)
				return
			}
		}
	})
	return a.initErr
}

// Run initialises the app and blocks until the context is cancelled or a
// SIGINT/SIGTERM is received, then performs ordered shutdown.
func (a *App) Run(ctx context.Context) error {
	if err := a.Init(ctx); err != nil {
		return err
	}

	signalCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	<-signalCtx.Done()
	// Use a fresh context for shutdown so cancellation from the signal does
	// not immediately abort cleanup steps. Callers wanting a bounded
	// shutdown should compose their own timeout context and call Shutdown
	// directly instead of Run.
	return a.Shutdown(context.Background())
}

// Shutdown invokes registered hooks in reverse order. Errors are joined so a
// failure in one hook does not skip the remainder.
func (a *App) Shutdown(ctx context.Context) error {
	a.mu.Lock()
	hooks := a.shutdowns
	a.shutdowns = nil
	a.mu.Unlock()

	var errs []error
	for i := len(hooks) - 1; i >= 0; i-- {
		if err := hooks[i](ctx); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("framework: shutdown: %w", errors.Join(errs...))
	}
	return nil
}
