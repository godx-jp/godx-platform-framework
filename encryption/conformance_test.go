package encryption

import (
	"context"
	"crypto/rand"
	"errors"
	"testing"

	edriver "github.com/godx-jp/godx-platform-framework/encryption/driver"
)

func TestConformanceCiphers(t *testing.T) {
	for _, name := range []string{edriver.DriverAESGCM, edriver.DriverChaCha20Poly1305} {
		name := name
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			cipher, err := edriver.New(ctx, edriver.Spec{Name: name})
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			if cipher.Name() != name {
				t.Fatalf("Name: %q", cipher.Name())
			}
			key := make([]byte, cipher.KeySize())
			if _, err := rand.Read(key); err != nil {
				t.Fatal(err)
			}

			plaintext := []byte("the rain in spain")
			sealed, err := cipher.Encrypt(ctx, key, plaintext)
			if err != nil {
				t.Fatalf("Encrypt: %v", err)
			}
			if len(sealed) <= len(plaintext) {
				t.Fatalf("sealed too short: %d <= %d", len(sealed), len(plaintext))
			}

			plain2, err := cipher.Decrypt(ctx, key, sealed)
			if err != nil {
				t.Fatalf("Decrypt: %v", err)
			}
			if string(plain2) != string(plaintext) {
				t.Fatalf("round trip mismatch: got %q", plain2)
			}

			// Two encryptions of the same plaintext under the same key
			// must differ due to random nonce.
			sealed2, _ := cipher.Encrypt(ctx, key, plaintext)
			if string(sealed) == string(sealed2) {
				t.Fatalf("AEAD nonce reuse — sealed outputs identical")
			}

			// Wrong key fails to authenticate.
			wrong := make([]byte, cipher.KeySize())
			wrong[0] = ^key[0]
			if _, err := cipher.Decrypt(ctx, wrong, sealed); !errors.Is(err, edriver.ErrAuthFailed) {
				t.Fatalf("expected ErrAuthFailed for wrong key, got %v", err)
			}

			// Tampered ciphertext fails.
			tampered := make([]byte, len(sealed))
			copy(tampered, sealed)
			tampered[len(tampered)-1] ^= 0xFF
			if _, err := cipher.Decrypt(ctx, key, tampered); !errors.Is(err, edriver.ErrAuthFailed) {
				t.Fatalf("expected ErrAuthFailed for tampered, got %v", err)
			}

			// Too-short input rejected.
			if _, err := cipher.Decrypt(ctx, key, []byte{0x00}); !errors.Is(err, edriver.ErrShortCiphertext) {
				t.Fatalf("expected ErrShortCiphertext, got %v", err)
			}

			// Wrong key size rejected.
			if _, err := cipher.Encrypt(ctx, key[:10], plaintext); !errors.Is(err, edriver.ErrInvalidKeySize) {
				t.Fatalf("expected ErrInvalidKeySize, got %v", err)
			}
			if _, err := cipher.Decrypt(ctx, key[:10], sealed); !errors.Is(err, edriver.ErrInvalidKeySize) {
				t.Fatalf("expected ErrInvalidKeySize, got %v", err)
			}
		})
	}
}
