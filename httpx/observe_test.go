package httpx

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestServe_ErrorObserver verifies that Serve notifies the process-global
// ErrorObserver with the correct (err, status) before writing the response,
// for both a *StatusError and a plain error, and that the response body/status
// is unchanged from the no-observer behavior.
func TestServe_ErrorObserver(t *testing.T) {
	cases := []struct {
		name       string
		handlerErr error
		wantStatus int
		wantBody   string // body produced by writeError
	}{
		{
			name:       "status error 503",
			handlerErr: WrapStatus(http.StatusServiceUnavailable, "down", errors.New("cause")),
			wantStatus: http.StatusServiceUnavailable,
			// >= 500 => plain http.Error with status text, message not leaked.
			wantBody: http.StatusText(http.StatusServiceUnavailable) + "\n",
		},
		{
			name:       "plain error maps to 500",
			handlerErr: errors.New("boom"),
			wantStatus: http.StatusInternalServerError,
			wantBody:   http.StatusText(http.StatusInternalServerError) + "\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var (
				gotCalled bool
				gotErr    error
				gotStatus int
				gotCtx    context.Context
			)
			SetErrorObserver(func(ctx context.Context, err error, status int) {
				gotCalled = true
				gotCtx = ctx
				gotErr = err
				gotStatus = status
			})
			t.Cleanup(func() { SetErrorObserver(nil) }) // don't leak global state

			h := Serve(func(w http.ResponseWriter, r *http.Request) error {
				return tc.handlerErr
			})
			rec := httptest.NewRecorder()
			h(rec, httptest.NewRequest(http.MethodGet, "/", nil))

			if !gotCalled {
				t.Fatalf("observer was not invoked")
			}
			if gotStatus != tc.wantStatus {
				t.Errorf("observer status = %d, want %d", gotStatus, tc.wantStatus)
			}
			if !errors.Is(gotErr, tc.handlerErr) {
				t.Errorf("observer err = %v, want %v", gotErr, tc.handlerErr)
			}
			if gotCtx == nil {
				t.Errorf("observer ctx is nil, want request context")
			}
			// Response unchanged vs. no-observer behavior.
			if rec.Code != tc.wantStatus {
				t.Errorf("response status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if rec.Body.String() != tc.wantBody {
				t.Errorf("response body = %q, want %q", rec.Body.String(), tc.wantBody)
			}
		})
	}
}

// TestServe_NoObserver verifies the default (nil observer) path is unchanged:
// no panic, correct status written.
func TestServe_NoObserver(t *testing.T) {
	SetErrorObserver(nil)
	h := Serve(func(w http.ResponseWriter, r *http.Request) error {
		return NewStatusError(http.StatusNotFound, "missing")
	})
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("code=%d, want 404", rec.Code)
	}
}

// TestStatusOf verifies statusOf mirrors the status writeError uses.
func TestStatusOf(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{
		{NewStatusError(http.StatusServiceUnavailable, "x"), http.StatusServiceUnavailable},
		{&StatusError{Code: 0, Message: "no code"}, http.StatusInternalServerError},
		{errors.New("plain"), http.StatusInternalServerError},
	}
	for _, tc := range cases {
		if got := statusOf(tc.err); got != tc.want {
			t.Errorf("statusOf(%v) = %d, want %d", tc.err, got, tc.want)
		}
	}
}
