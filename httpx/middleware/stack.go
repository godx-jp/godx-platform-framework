package middleware

import (
	"net/http"
)

// Stack composes multiple middleware layers outermost-first.
func Stack(layers ...func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(final http.Handler) http.Handler {
		for i := len(layers) - 1; i >= 0; i-- {
			if layers[i] == nil {
				continue
			}
			final = layers[i](final)
		}
		return final
	}
}
