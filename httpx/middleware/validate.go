package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/godx-jp/godx-platform-framework/httpx"
	"github.com/godx-jp/godx-platform-framework/validation"
)

type validatedKey struct{}

// ValidateJSON decodes the request body into a fresh DTO from factory,
// runs ValidateStruct, and stores the value on the request context.
// Handlers retrieve it with Validated[T].
//
// The request body is capped at [httpx.DefaultMaxBodyBytes] before decoding
// to protect against memory-exhaustion DoS. Decode failures return a generic
// 400 (or 413 when the body is too large) without echoing decoder internals;
// validation failures return a structured 422 listing field + machine code.
func ValidateJSON(v *validation.Validator, factory func() any) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			dst := factory()
			r.Body = http.MaxBytesReader(nil, r.Body, httpx.DefaultMaxBodyBytes)
			if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
				var mbe *http.MaxBytesError
				if errors.As(err, &mbe) {
					writeValidationError(w, http.StatusRequestEntityTooLarge, "request body too large")
					return
				}
				writeValidationError(w, http.StatusBadRequest, "invalid json")
				return
			}
			if err := v.ValidateStruct(r.Context(), dst); err != nil {
				writeFieldErrors(w, err)
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

// fieldErrorView is the stable, client-safe shape of one validation
// failure. It exposes only the public field name and a machine-readable
// rule code — never the raw Go error text or any internal struct detail.
type fieldErrorView struct {
	Field string `json:"field"`
	Code  string `json:"code"`
	Param string `json:"param,omitempty"`
}

// writeFieldErrors marshals typed validation errors into a stable JSON
// envelope. If err is not a validation.Errors (e.g. ErrNotStruct), a
// generic 422 is returned without leaking the underlying error string.
func writeFieldErrors(w http.ResponseWriter, err error) {
	var verrs validation.Errors
	if !errors.As(err, &verrs) || !verrs.Has() {
		writeValidationError(w, http.StatusUnprocessableEntity, "validation failed")
		return
	}
	views := make([]fieldErrorView, 0, len(verrs))
	for _, fe := range verrs {
		views = append(views, fieldErrorView{
			Field: fe.Field,
			Code:  fe.Rule,
			Param: fe.Param,
		})
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusUnprocessableEntity)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error":  "validation failed",
		"fields": views,
	})
}

func writeValidationError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
