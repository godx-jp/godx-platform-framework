package driver

import "errors"

var (
	ErrClosed         = errors.New("messaging: broker closed")
	ErrNotImplemented = errors.New("messaging: driver not implemented")
)
