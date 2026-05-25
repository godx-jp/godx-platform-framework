package driver

import "errors"

var (
	ErrClosed          = errors.New("httpclient: client is closed")
	ErrCircuitOpen     = errors.New("httpclient: circuit breaker open")
	ErrInvalidBaseURL  = errors.New("httpclient: invalid base URL")
)
