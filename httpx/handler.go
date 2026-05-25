package httpx

import (
	"encoding/json"
	"errors"
	"net/http"
)

// HandlerFunc is the preferred handler signature — return nil on success
// or an error to let the framework write an appropriate response.
type HandlerFunc func(w http.ResponseWriter, r *http.Request) error

// StatusError carries an HTTP status and optional wrapped cause.
type StatusError struct {
	Code    int
	Message string
	Err     error
}

func (e *StatusError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return http.StatusText(e.Code)
}

func (e *StatusError) Unwrap() error { return e.Err }

// NewStatusError builds a StatusError.
func NewStatusError(code int, message string) *StatusError {
	return &StatusError{Code: code, Message: message}
}

// WrapStatus wraps err with code and message.
func WrapStatus(code int, message string, err error) *StatusError {
	return &StatusError{Code: code, Message: message, Err: err}
}

// Serve converts a HandlerFunc into an http.HandlerFunc with unified
// error handling.
func Serve(h HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := h(w, r); err != nil {
			writeError(w, err)
		}
	}
}

func writeError(w http.ResponseWriter, err error) {
	var se *StatusError
	if errors.As(err, &se) && se.Code > 0 {
		if se.Code >= 500 {
			http.Error(w, http.StatusText(se.Code), se.Code)
			return
		}
		JSON(w, se.Code, map[string]string{"error": se.Error()})
		return
	}
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

// JSON writes v as JSON with the given status code.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// NoContent responds with status and an empty body.
func NoContent(w http.ResponseWriter, status int) {
	w.WriteHeader(status)
}

// DecodeJSON decodes the request body into dst. Returns 400 on failure.
func DecodeJSON(r *http.Request, dst any) error {
	if r.Body == nil {
		return NewStatusError(http.StatusBadRequest, "empty body")
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return WrapStatus(http.StatusBadRequest, "invalid json", err)
	}
	return nil
}
