package driver

import "errors"

// ErrClosed is returned by a Channel after Shutdown.
var ErrClosed = errors.New("notifications: channel is closed")
