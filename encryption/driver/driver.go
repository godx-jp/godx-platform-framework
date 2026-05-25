// Package driver is the public contract for symmetric encryption
// implementations. All ciphers are authenticated AEADs — encrypt
// returns nonce|ciphertext|tag; decrypt verifies the tag before
// returning plaintext.
package driver

import "context"

// Cipher is the in-process behaviour of one AEAD-style cipher.
// Implementations must be safe for concurrent use.
type Cipher interface {
	// Name returns the canonical driver name (aesgcm, chacha20poly1305).
	Name() string

	// KeySize returns the required key length in bytes (32 for aesgcm
	// and chacha20poly1305). Manager uses this when adding keys.
	KeySize() int

	// Encrypt seals plaintext with key. nonce is generated internally
	// (random per call). Output is nonce||ciphertext||tag in that
	// order — drivers must document the layout, but Manager treats
	// the output opaquely.
	Encrypt(ctx context.Context, key, plaintext []byte) (sealed []byte, err error)

	// Decrypt opens sealed with key. Returns ErrAuthFailed when the
	// AEAD tag does not verify (wrong key, tampered ciphertext, or
	// truncated input).
	Decrypt(ctx context.Context, key, sealed []byte) (plaintext []byte, err error)
}

// Constructor builds a Cipher from a Spec. Each driver package
// exports one and registers it at init time.
type Constructor func(ctx context.Context, spec Spec) (Cipher, error)
