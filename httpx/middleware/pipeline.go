package middleware

import (
	"net/http"

	"github.com/godx-jp/godx-platform-framework/pipeline"
)

// Pipeline composes HTTPStages using the pipeline module's Chain helper.
// Stages run outermost-first (first argument runs first on the request).
func Pipeline(stages ...pipeline.HTTPStage) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return pipeline.Chain(next, stages...)
	}
}
