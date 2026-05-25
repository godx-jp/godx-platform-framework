// Package aesgcm provides the AES-256-GCM Cipher driver — Laravel's
// default Crypt driver (functionally equivalent to PHP's
// openssl_encrypt with aes-256-gcm).
package aesgcm

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"

	edriver "github.com/godx-jp/godx-platform-framework/encryption/driver"
)

const (
	// KeySize is 32 bytes (AES-256).
	KeySize = 32
	// NonceSize is 12 bytes (GCM standard).
	NonceSize = 12
)

func init() {
	edriver.Register(edriver.DriverAESGCM, func(_ context.Context, _ edriver.Spec) (edriver.Cipher, error) {
		return New(), nil
	})
}

// New constructs a fresh AES-256-GCM Cipher. Cipher is stateless;
// the same instance can serve many keys.
func New() edriver.Cipher { return c{} }

type c struct{}

func (c) Name() string { return edriver.DriverAESGCM }
func (c) KeySize() int { return KeySize }

func (c) Encrypt(_ context.Context, key, plaintext []byte) ([]byte, error) {
	if len(key) != KeySize {
		return nil, fmt.Errorf("%w: aesgcm wants %d, got %d", edriver.ErrInvalidKeySize, KeySize, len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("encryption/aesgcm: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("encryption/aesgcm: %w", err)
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("encryption/aesgcm: nonce: %w", err)
	}
	// Layout: nonce || ciphertext+tag (GCM Seal appends the tag).
	sealed := aead.Seal(nonce, nonce, plaintext, nil)
	return sealed, nil
}

func (c) Decrypt(_ context.Context, key, sealed []byte) ([]byte, error) {
	if len(key) != KeySize {
		return nil, fmt.Errorf("%w: aesgcm wants %d, got %d", edriver.ErrInvalidKeySize, KeySize, len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("encryption/aesgcm: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("encryption/aesgcm: %w", err)
	}
	ns := aead.NonceSize()
	if len(sealed) < ns+aead.Overhead() {
		return nil, edriver.ErrShortCiphertext
	}
	nonce := sealed[:ns]
	body := sealed[ns:]
	plain, err := aead.Open(nil, nonce, body, nil)
	if err != nil {
		// crypto/cipher's Open returns "message authentication
		// failed" on any tag mismatch; normalise to a sentinel.
		return nil, edriver.ErrAuthFailed
	}
	return plain, nil
}
