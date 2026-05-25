package driver

import "errors"

// ErrClosed is returned by a Transport after Shutdown.
var ErrClosed = errors.New("mail: transport is closed")
