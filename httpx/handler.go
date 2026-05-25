package httpx

import (
	"encoding/json"
	"errors"
	"net/http"
)

// DefaultMaxBodyBytes is the default request-body cap applied by
// [DecodeJSON]. Bodies larger than this are rejected before they are
// buffered, which protects against memory-exhaustion DoS from
// unbounded JSON payloads. Override per-call with [DecodeJSONLimit].
const DefaultMaxBodyBytes int64 = 1 << 20 // 1 MiB

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
			// Notify the process-global observer (if set) with the status that
			// writeError will use, BEFORE writing the response, passing the
			// request context so the observer can record on the active span.
			if obs := errorObserver; obs != nil {
				obs(r.Context(), err, statusOf(err))
			}
			writeError(w, err)
		}
	}
}

// statusOf returns the HTTP status writeError will write for err: the
// *StatusError code when it is > 0, otherwise http.StatusInternalServerError.
// writeError is defined in terms of statusOf so the two never diverge.
func statusOf(err error) int {
	var se *StatusError
	if errors.As(err, &se) && se.Code > 0 {
		return se.Code
	}
	return http.StatusInternalServerError
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

// DecodeJSON decodes the request body into dst using the default body-size
// cap ([DefaultMaxBodyBytes]). Returns 400 on a malformed body and 413 when
// the body exceeds the limit.
func DecodeJSON(r *http.Request, dst any) error {
	return DecodeJSONLimit(r, dst, DefaultMaxBodyBytes)
}

// DecodeJSONLimit decodes the request body into dst, rejecting bodies larger
// than limit bytes. Pass a non-positive limit to use [DefaultMaxBodyBytes].
//
// Unknown fields are rejected. A malformed body returns a 400 StatusError; a
// body that exceeds the limit returns a 413 StatusError so handlers do not
// surface it as a 500.
func DecodeJSONLimit(r *http.Request, dst any, limit int64) error {
	if r.Body == nil {
		return NewStatusError(http.StatusBadRequest, "empty body")
	}
	if limit <= 0 {
		limit = DefaultMaxBodyBytes
	}
	r.Body = http.MaxBytesReader(nil, r.Body, limit)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			return WrapStatus(http.StatusRequestEntityTooLarge, "request body too large", err)
		}
		return WrapStatus(http.StatusBadRequest, "invalid json", err)
	}
	return nil
}
