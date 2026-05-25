// Run with `go run ./examples/httpx` from the repo root.
package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"

	"github.com/godx-jp/godx-platform-framework/framework"
	"github.com/godx-jp/godx-platform-framework/httpx"
	hmw "github.com/godx-jp/godx-platform-framework/httpx/middleware"
	"github.com/godx-jp/godx-platform-framework/observability"
	omw "github.com/godx-jp/godx-platform-framework/observability/middleware"
	"github.com/godx-jp/godx-platform-framework/ratelimit"
	"github.com/godx-jp/godx-platform-framework/validation"
)

type createUserBody struct {
	Email string `json:"email" validate:"required,email"`
	Name  string `json:"name" validate:"required,min=2"`
}

func main() {
	ctx := context.Background()
	app := framework.New("httpx-example", "0.0.0").
		Use(observability.Module).
		Use(validation.Module).
		Use(ratelimit.Module)

	if err := app.Init(ctx); err != nil {
		log.Fatalf("init: %v", err)
	}
	defer func() { _ = app.Shutdown(ctx) }()

	v, _ := validation.FromApp(app)
	rl, _ := ratelimit.FromApp(app)
	obs := observability.FromApp(app)

	r := httpx.NewRouter()
	r.Use(omw.HTTP(obs))
	r.Use(hmw.RateLimitByIP(rl.Default()))
	r.Use(hmw.Pipeline(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			obs.Logger().InfoContext(req.Context(), "incoming", "path", req.URL.Path)
			next.ServeHTTP(w, req)
		})
	}))

	r.Group(func(sub chi.Router) {
		sub.Use(hmw.ValidateJSON(v, func() any { return &createUserBody{} }))
		httpx.Route(sub, http.MethodPost, "/users", createUser)
	})

	httpx.Route(r, http.MethodGet, "/healthz", func(w http.ResponseWriter, r *http.Request) error {
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return nil
	})

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	log.Printf("listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}

func createUser(w http.ResponseWriter, r *http.Request) error {
	body, ok := hmw.Validated[*createUserBody](r)
	if !ok {
		return httpx.NewStatusError(http.StatusInternalServerError, "missing validated body")
	}
	httpx.JSON(w, http.StatusCreated, map[string]string{
		"email": body.Email,
		"name":  body.Name,
	})
	return nil
}
