# Minimal example

Smallest possible program using `framework` + `observability`. No HTTP, no infrastructure.

```bash
OBSERVABILITY_DRIVER=stdout go run .
```

Expected output (one JSON line):

```json
{"time":"...","level":"INFO","msg":"hello from godx-platform-framework","service":"minimal","version":"0.1.0","env":"dev","trace_id":"...32-hex...","span_id":"...16-hex...","driver":"stdout","hint":"trace_id is auto-injected"}
```

Try switching driver without changing code:

```bash
# OTLP (needs an OTel Collector listening on :4317)
OBSERVABILITY_DRIVER=otlp \
OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317 \
go run .

# File (Laravel-style; rotates daily by default)
OBSERVABILITY_DRIVER=file \
OBSERVABILITY_LOG_FILE_PATH=./logs/app.log \
go run .
```

The program is identical; only the env vars change.
