// Package chacha20poly1305 provides the ChaCha20-Poly1305 Cipher
// driver. ChaCha is typically faster than AES on hardware without
// AES-NI (older ARM, some embedded chips). 32-byte key.
package chacha20poly1305

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"

	chacha "golang.org/x/crypto/chacha20poly1305"

	edriver "github.com/godx-jp/godx-platform-framework/encryption/driver"
)

const KeySize = chacha.KeySize

func init() {
	edriver.Register(edriver.DriverChaCha20Poly1305, func(_ context.Context, _ edriver.Spec) (edriver.Cipher, error) {
		return New(), nil
	})
}

// New constructs a ChaCha20-Poly1305 Cipher. Stateless; reusable.
func New() edriver.Cipher { return c{} }

type c struct{}

func (c) Name() string { return edriver.DriverChaCha20Poly1305 }
func (c) KeySize() int { return KeySize }

func (c) Encrypt(_ context.Context, key, plaintext []byte) ([]byte, error) {
	if len(key) != KeySize {
		return nil, fmt.Errorf("%w: chacha20poly1305 wants %d, got %d", edriver.ErrInvalidKeySize, KeySize, len(key))
	}
	aead, err := chacha.New(key)
	if err != nil {
		return nil, fmt.Errorf("encryption/chacha20poly1305: %w", err)
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("encryption/chacha20poly1305: nonce: %w", err)
	}
	sealed := aead.Seal(nonce, nonce, plaintext, nil)
	return sealed, nil
}

func (c) Decrypt(_ context.Context, key, sealed []byte) ([]byte, error) {
	if len(key) != KeySize {
		return nil, fmt.Errorf("%w: chacha20poly1305 wants %d, got %d", edriver.ErrInvalidKeySize, KeySize, len(key))
	}
	aead, err := chacha.New(key)
	if err != nil {
		return nil, fmt.Errorf("encryption/chacha20poly1305: %w", err)
	}
	ns := aead.NonceSize()
	if len(sealed) < ns+aead.Overhead() {
		return nil, edriver.ErrShortCiphertext
	}
	nonce := sealed[:ns]
	body := sealed[ns:]
	plain, err := aead.Open(nil, nonce, body, nil)
	if err != nil {
		return nil, edriver.ErrAuthFailed
	}
	return plain, nil
}
