package driver

import "errors"

// ErrClosed is returned by a Limiter after Shutdown.
var ErrClosed = errors.New("ratelimit: limiter is closed")
