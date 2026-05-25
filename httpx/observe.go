package httpx

import "context"

// ErrorObserver is notified of every non-nil error returned by a HandlerFunc,
// with the HTTP status that will be written, BEFORE the response is sent. Set
// once at startup. nil (default) preserves today's behavior. Implementations
// must not write to the response.
type ErrorObserver func(ctx context.Context, err error, status int)

// errorObserver holds the process-global observer. It is set once at startup
// via SetErrorObserver (mirroring SetProblemTypeBaseURL / SetRequestIDExtractor
// in problem.go) and read on every error path; a plain package var read is fine
// under the set-once-at-startup contract.
var errorObserver ErrorObserver

// SetErrorObserver installs the process-global [ErrorObserver]. Call once at
// startup, before serving traffic. Passing nil clears any previously installed
// observer and restores the default (no-op) behavior.
//
// This is the bridge that lets the observability stack record an error against
// the active span without httpx taking a hard dependency on observability:
// the observer is a plain func value the caller wires up.
func SetErrorObserver(fn ErrorObserver) {
	errorObserver = fn
}
