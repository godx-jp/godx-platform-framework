# HTTPX

> chi router helpers, HandlerFunc conventions, and middleware integrating pipeline, validation, and ratelimit.

## Handler conventions

```go
type HandlerFunc func(w http.ResponseWriter, r *http.Request) error

httpx.Route(r, http.MethodGet, "/ping", func(w http.ResponseWriter, r *http.Request) error {
    httpx.JSON(w, http.StatusOK, map[string]string{"pong": "1"})
    return nil
})
```

Return `httpx.NewStatusError(code, msg)` for controlled error responses.

## Middleware stack

```go
r := httpx.NewRouter()
r.Use(hmw.RateLimitByIP(rl.Default()))
r.Use(hmw.Pipeline(loggingStage))
r.Group(func(sub chi.Router) {
    sub.Use(hmw.ValidateJSON(v, func() any { return &CreateUser{} }))
    httpx.Route(sub, http.MethodPost, "/users", createUser)
})
```

| Middleware | Module |
|------------|--------|
| `hmw.Pipeline` | `pipeline` |
| `hmw.ValidateJSON` | `validation` |
| `hmw.RateLimitByIP` | `ratelimit` |

See [examples/httpx/main.go](../../examples/httpx/main.go).
