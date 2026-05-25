package driver

import "errors"

// ErrNotSupported is returned when a Source does not implement an
// optional capability (currently used by drivers that do not
// implement Watcher and want to surface that fact explicitly).
var ErrNotSupported = errors.New("config: driver does not support this operation")

// ErrClosed is returned by a Source after Shutdown has been called.
var ErrClosed = errors.New("config: source is closed")

// ErrFileMissing is returned by the file driver when Optional is
// false and the configured Path does not exist. Wrapped with %w so
// callers can errors.Is for it when deciding to bootstrap defaults.
var ErrFileMissing = errors.New("config: file source path does not exist")

// ErrUnsupportedFormat is returned by the file driver when the file
// extension is not recognised and Spec.Format is empty.
var ErrUnsupportedFormat = errors.New("config: unsupported file format (expected .yaml/.yml/.json/.toml or explicit Format)")
