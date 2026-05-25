package httpx

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// OpenAPIConfig configures Swagger UI routes on a chi router.
//
// Registers (no auth — mount before JWT middleware):
//   - GET /docs          → redirect /docs/
//   - GET /docs/         → Swagger UI HTML
//   - GET /docs/openapi.yaml or /docs/openapi.json → raw contract (auto from spec bytes)
type OpenAPIConfig struct {
	Title string
	Spec  []byte
}

// MountOpenAPI attaches Swagger UI + raw OpenAPI spec handlers to r.
func MountOpenAPI(r chi.Router, cfg OpenAPIConfig) {
	if len(cfg.Spec) == 0 {
		return
	}
	title := cfg.Title
	if title == "" {
		title = "API"
	}
	specPath := openAPISpecPath(cfg.Spec)

	r.Get("/docs", func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, "/docs/", http.StatusMovedPermanently)
	})
	r.Get(specPath, func(w http.ResponseWriter, req *http.Request) {
		serveOpenAPISpec(w, cfg.Spec)
	})
	r.Get("/docs/", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(swaggerUIHTML(title, specPath)))
	})
}

func openAPISpecPath(spec []byte) string {
	if isJSONSpec(spec) {
		return "/docs/openapi.json"
	}
	return "/docs/openapi.yaml"
}

func isJSONSpec(spec []byte) bool {
	for _, b := range spec {
		switch b {
		case ' ', '\n', '\r', '\t':
			continue
		}
		return b == '{'
	}
	return false
}

func serveOpenAPISpec(w http.ResponseWriter, spec []byte) {
	ct := "application/yaml; charset=utf-8"
	if isJSONSpec(spec) {
		ct = "application/json; charset=utf-8"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	_, _ = w.Write(spec)
}

func swaggerUIHTML(title, specPath string) string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>` + title + ` — API Docs</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
  <style>body { margin: 0; }</style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-standalone-preset.js"></script>
  <script>
    window.onload = () => {
      window.ui = SwaggerUIBundle({
        url: "` + specPath + `",
        dom_id: "#swagger-ui",
        deepLinking: true,
        presets: [SwaggerUIBundle.presets.apis, SwaggerUIStandalonePreset],
        layout: "StandaloneLayout",
        persistAuthorization: true,
        displayRequestDuration: true,
      });
    };
  </script>
</body>
</html>`
}
