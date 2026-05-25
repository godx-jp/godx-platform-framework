// Package httpclient provides a Laravel-style HTTP client facade with
// swappable drivers (stdlib, mock, resilient), optional OTel spans on
// every outbound request, and convenience helpers (Get, Post, JSON).
//
//	app := framework.New("svc", "1.0.0").Use(httpclient.Module)
//	_ = app.Init(ctx)
//	mgr, _ := httpclient.FromApp(app)
//	resp, err := mgr.Get(ctx, "https://api.example/users/1")
//
// Drivers:
//
//	stdlib    - net/http.Client wrapper (light, auto)
//	mock      - in-memory recorder for tests (light, auto)
//	resilient - retry + backoff + circuit breaker on stdlib (light, auto)
//
// OTel: when an observability Provider is on the App, outbound calls
// emit client spans automatically via the otel RoundTripper wrapper.
package httpclient
