package driver

import "errors"

// ErrNotSupported is returned when a driver does not implement a
// particular optional capability (currently none — reserved for future
// expansion such as tagging).
var ErrNotSupported = errors.New("cache: driver does not support this operation")

// ErrNotImplemented is returned by stub drivers in the registry that
// have a name but no real implementation yet. Currently unused (every
// shipped driver is fully implemented in v0.7.0).
var ErrNotImplemented = errors.New("cache: driver not implemented")

// ErrNotInteger is returned by Increment / Decrement when the value
// already stored under the key cannot be parsed as a decimal int64.
var ErrNotInteger = errors.New("cache: value is not an integer counter")
