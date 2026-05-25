package driver

import "errors"

// ErrNotFound is returned by Get / Forget when the key does not
// exist. Use errors.Is to branch on missing-key paths.
var ErrNotFound = errors.New("secrets: key not found")

// ErrReadOnly is returned by Put / Forget for backends that don't
// support writes (e.g. env-vars-only driver in production-only mode).
var ErrReadOnly = errors.New("secrets: backend is read-only")

// ErrListNotSupported is returned by List for backends that cannot
// enumerate (the env driver, for example).
var ErrListNotSupported = errors.New("secrets: backend does not support listing")

// ErrClosed is returned by a Store after Shutdown.
var ErrClosed = errors.New("secrets: store is closed")
