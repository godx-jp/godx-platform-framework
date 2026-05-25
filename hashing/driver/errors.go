package driver

import "errors"

// ErrInvalidHash is returned by Check / Info when the encoded hash
// is malformed (missing fields, wrong segment count, bad base64).
var ErrInvalidHash = errors.New("hashing: invalid encoded hash")

// ErrUnknownFormat is returned by Info when the hasher does not
// recognise the encoded hash. Callers can fall through to the
// registry to find a hasher that does.
var ErrUnknownFormat = errors.New("hashing: hash format not understood by this driver")

// ErrPasswordTooLong is returned by Make when plain exceeds the
// driver's hard limit (bcrypt: 72 bytes).
var ErrPasswordTooLong = errors.New("hashing: password too long for this driver")

// ErrIncompatibleParams is returned by Make when a Spec field is
// outside the driver's accepted range (e.g. bcrypt cost > 31).
var ErrIncompatibleParams = errors.New("hashing: spec parameters out of range")
