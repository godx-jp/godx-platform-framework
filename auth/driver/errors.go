package driver

import "errors"

// ErrNotImplemented is returned by drivers that are registered but not yet implemented.
var ErrNotImplemented = errors.New("auth: not implemented")

// ErrInvalidCredential is returned when credentials are missing or invalid.
var ErrInvalidCredential = errors.New("auth: invalid credential")

// ErrClosed is returned by a Guard after Shutdown.
var ErrClosed = errors.New("auth: guard is closed")
