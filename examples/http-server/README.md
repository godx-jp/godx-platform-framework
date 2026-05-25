# HTTP server example

A minimal `net/http` server wired with the framework observability middleware. Demonstrates:

- App lifecycle (`framework.New` → `Init` → `OnShutdown` → `Run`).
- Per-request trace, correlation ID propagation, structured log line per request.
- Reading the provider from request context inside a handler.

## Run

```bash
OBSERVABILITY_DRIVER=stdout go run .

# Another terminal:
curl -i http://localhost:8080/hello
curl -i -H 'X-Correlation-ID: my-trace-1' http://localhost:8080/hello
```

You'll see one `http_request` JSON line per request (logged by the middleware) and one `answering hello` line per request (logged by the handler) — both sharing the same `trace_id`.

## Switch driver

```bash
# File — Laravel-style local file, daily rotation, 14-day retention, gzip
OBSERVABILITY_DRIVER=file \
OBSERVABILITY_LOG_FILE_PATH=./logs/app.log \
go run .

# OTLP — needs an OTel Collector on :4317 (run godx-platform-observability with `make up`)
OBSERVABILITY_DRIVER=otlp \
OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317 \
go run .
```
