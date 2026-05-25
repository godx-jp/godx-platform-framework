package driver

import "errors"

// ErrNotFound is returned by drivers when an operation references a key
// that does not exist on the backend. Callers should test with
// errors.Is(err, driver.ErrNotFound).
var ErrNotFound = errors.New("storage: key not found")

// ErrNotSupported is returned by drivers for capabilities they do not
// implement (e.g. SignedURL on the memory driver). Capability-aware
// callers should test with errors.Is(err, driver.ErrNotSupported) and
// degrade gracefully.
var ErrNotSupported = errors.New("storage: operation not supported by driver")

// ErrNotImplemented is returned by stub drivers (s3, gcs, azure, minio
// at v0.6.0) until their full implementation lands. Distinct from
// ErrNotSupported so that callers can distinguish "this driver will
// never do this" from "this driver does not do this yet".
var ErrNotImplemented = errors.New("storage: driver not yet implemented")
