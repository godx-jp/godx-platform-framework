package encryption

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"

	edriver "github.com/godx-jp/godx-platform-framework/encryption/driver"
)

// tokenVersion is the prefix that lets us evolve the encoded
// format in the future. Today there is only v1.
const tokenVersion = "v1"

// Encrypter is the user-facing facade — Laravel's Crypt. Wraps a
// Cipher plus a versioned key ring so old ciphertext stays
// decryptable through key rotations.
type Encrypter struct {
	mu      sync.RWMutex
	cipher  edriver.Cipher
	primary string                // key id of the active key
	keys    map[string][]byte    // id → raw key
}

// NewEncrypter constructs an Encrypter wrapping cipher. Add at
// least one key with AddKey before encrypting.
func NewEncrypter(cipher edriver.Cipher) *Encrypter {
	return &Encrypter{cipher: cipher, keys: map[string][]byte{}}
}

// CipherName returns the underlying cipher's name (mainly diagnostics).
func (e *Encrypter) CipherName() string { return e.cipher.Name() }

// AddKey registers key under id. id must be non-empty and not yet
// registered. Returns an error when the key length does not match
// the cipher.
func (e *Encrypter) AddKey(id string, key []byte) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("encryption: AddKey: id is required")
	}
	if len(key) != e.cipher.KeySize() {
		return fmt.Errorf("%w: cipher %q wants %d bytes, got %d", edriver.ErrInvalidKeySize, e.cipher.Name(), e.cipher.KeySize(), len(key))
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.keys[id]; exists {
		return fmt.Errorf("encryption: key id %q already registered", id)
	}
	e.keys[id] = append([]byte(nil), key...)
	if e.primary == "" {
		e.primary = id
	}
	return nil
}

// SetPrimary flags id as the encryption key. id must already be
// registered via AddKey. Existing tokens encrypted under prior keys
// are still decryptable.
func (e *Encrypter) SetPrimary(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.keys[id]; !ok {
		return fmt.Errorf("encryption: SetPrimary(%q): key is not registered", id)
	}
	e.primary = id
	return nil
}

// PrimaryKeyID returns the id of the currently active key.
func (e *Encrypter) PrimaryKeyID() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.primary
}

// KeyIDs returns the registered key ids (unsorted; insertion-order is
// not preserved).
func (e *Encrypter) KeyIDs() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]string, 0, len(e.keys))
	for id := range e.keys {
		out = append(out, id)
	}
	return out
}

// Encrypt seals plaintext with the primary key and returns the encoded
// token "v1:<key-id>:<base64(nonce|ciphertext|tag)>".
func (e *Encrypter) Encrypt(ctx context.Context, plaintext []byte) (string, error) {
	e.mu.RLock()
	id := e.primary
	key, ok := e.keys[id]
	e.mu.RUnlock()
	if !ok || id == "" {
		return "", fmt.Errorf("encryption: no primary key registered")
	}
	sealed, err := e.cipher.Encrypt(ctx, key, plaintext)
	if err != nil {
		return "", err
	}
	return tokenVersion + ":" + id + ":" + base64.RawURLEncoding.EncodeToString(sealed), nil
}

// EncryptString is the string-typed convenience wrapper.
func (e *Encrypter) EncryptString(ctx context.Context, plain string) (string, error) {
	return e.Encrypt(ctx, []byte(plain))
}

// Decrypt resolves the token's key-id against the key ring and
// returns the plaintext.
func (e *Encrypter) Decrypt(ctx context.Context, token string) ([]byte, error) {
	id, sealed, err := decodeToken(token)
	if err != nil {
		return nil, err
	}
	e.mu.RLock()
	key, ok := e.keys[id]
	e.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %q", edriver.ErrUnknownKey, id)
	}
	plain, err := e.cipher.Decrypt(ctx, key, sealed)
	if err != nil {
		return nil, err
	}
	return plain, nil
}

// DecryptString is the string-typed convenience wrapper.
func (e *Encrypter) DecryptString(ctx context.Context, token string) (string, error) {
	plain, err := e.Decrypt(ctx, token)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

// KeyIDOf parses the encoded token and returns the key id it was
// encrypted under. Useful for rotation tooling that wants to skip
// already-current rows.
func KeyIDOf(token string) (string, error) {
	id, _, err := decodeToken(token)
	return id, err
}

func decodeToken(token string) (string, []byte, error) {
	parts := strings.SplitN(token, ":", 3)
	if len(parts) != 3 {
		return "", nil, edriver.ErrInvalidToken
	}
	if parts[0] != tokenVersion {
		return "", nil, fmt.Errorf("%w: unsupported version %q", edriver.ErrInvalidToken, parts[0])
	}
	if parts[1] == "" {
		return "", nil, fmt.Errorf("%w: empty key id", edriver.ErrInvalidToken)
	}
	sealed, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return "", nil, fmt.Errorf("%w: %w", edriver.ErrInvalidToken, err)
	}
	return parts[1], sealed, nil
}

// ParseKey decodes a key string. Accepts:
//
//	base64:<RawStdBase64>      - Laravel-format (after the "base64:" prefix)
//	hex:<hex>                  - hex-encoded
//	raw bytes (32 ASCII chars) - treated as a literal key
func ParseKey(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	switch {
	case strings.HasPrefix(s, "base64:"):
		key, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(s, "base64:"))
		if err != nil {
			return nil, fmt.Errorf("encryption: parse base64 key: %w", err)
		}
		return key, nil
	case strings.HasPrefix(s, "hex:"):
		return decodeHex(strings.TrimPrefix(s, "hex:"))
	default:
		return []byte(s), nil
	}
}

func decodeHex(s string) ([]byte, error) {
	if len(s)%2 != 0 {
		return nil, errors.New("encryption: hex key has odd length")
	}
	out := make([]byte, len(s)/2)
	for i := 0; i < len(out); i++ {
		hi, err := unhex(s[i*2])
		if err != nil {
			return nil, err
		}
		lo, err := unhex(s[i*2+1])
		if err != nil {
			return nil, err
		}
		out[i] = hi<<4 | lo
	}
	return out, nil
}

func unhex(b byte) (byte, error) {
	switch {
	case b >= '0' && b <= '9':
		return b - '0', nil
	case b >= 'a' && b <= 'f':
		return b - 'a' + 10, nil
	case b >= 'A' && b <= 'F':
		return b - 'A' + 10, nil
	}
	return 0, fmt.Errorf("encryption: invalid hex char %q", b)
}
