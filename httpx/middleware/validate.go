package middleware

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/godx-jp/godx-platform-framework/validation"
)

type validatedKey struct{}

// ValidateJSON decodes the request body into a fresh DTO from factory,
// runs ValidateStruct, and stores the value on the request context.
// Handlers retrieve it with Validated[T].
func ValidateJSON(v *validation.Validator, factory func() any) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			dst := factory()
			if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
				writeValidationError(w, http.StatusBadRequest, "invalid json")
				return
			}
			if err := v.ValidateStruct(r.Context(), dst); err != nil {
				writeValidationError(w, http.StatusUnprocessableEntity, err.Error())
				return
			}
			ctx := context.WithValue(r.Context(), validatedKey{}, dst)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Validated returns the validated DTO stored by ValidateJSON middleware.
func Validated[T any](r *http.Request) (T, bool) {
	var zero T
	v, ok := r.Context().Value(validatedKey{}).(T)
	if !ok {
		return zero, false
	}
	return v, true
}

func writeValidationError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
