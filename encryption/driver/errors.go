package driver

import "errors"

// ErrAuthFailed is returned by Decrypt when the AEAD tag does not
// verify. Callers must treat this as "tampered or wrong key" — never
// log the ciphertext on this path.
var ErrAuthFailed = errors.New("encryption: authentication failed (wrong key or tampered ciphertext)")

// ErrInvalidKeySize is returned when the supplied key length does
// not match the cipher's required size.
var ErrInvalidKeySize = errors.New("encryption: invalid key size for cipher")

// ErrShortCiphertext is returned by Decrypt when sealed is shorter
// than the nonce + tag headers.
var ErrShortCiphertext = errors.New("encryption: ciphertext is shorter than the AEAD overhead")

// ErrInvalidToken is returned by Manager.Decrypt when the encoded
// token cannot be parsed (wrong version prefix, missing key id, bad
// base64).
var ErrInvalidToken = errors.New("encryption: invalid encoded token")

// ErrUnknownKey is returned by Manager.Decrypt when the token's
// key-id is not present in the key ring.
var ErrUnknownKey = errors.New("encryption: ciphertext refers to an unknown key id")
