# Minimal example

Smallest possible program using `framework` + `observability`. No HTTP, no infrastructure.

```bash
OBS_BACKEND=stdout go run .
```

Expected output (one JSON line):

```json
{"time":"...","level":"INFO","msg":"hello from godx-platform-framework","service":"minimal","version":"0.1.0","env":"dev","trace_id":"...32-hex...","span_id":"...16-hex...","backend":"stdout","hint":"trace_id is auto-injected"}
```

Try switching backend without changing code:

```bash
# OTLP (needs an OTel Collector listening on :4317)
OBS_BACKEND=otlp \
OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317 \
go run .
```

The program is identical; only the env vars change.
