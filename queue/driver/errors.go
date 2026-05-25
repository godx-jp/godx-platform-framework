package driver

import "errors"

var (
	ErrNotFound       = errors.New("queue: job not found")
	ErrClosed         = errors.New("queue: backend is closed")
	ErrNotImplemented = errors.New("queue: driver not implemented")
)
