// Package scrypt provides the scrypt Hasher driver. Encoded output
// uses a custom $scrypt$ string format that captures N, r, p, salt,
// and digest so verification and rehash decisions can be made from
// the hash alone.
package scrypt

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/scrypt"

	hdriver "github.com/godx-jp/godx-platform-framework/hashing/driver"
)

func init() {
	hdriver.Register(hdriver.DriverScrypt, func(_ context.Context, spec hdriver.Spec) (hdriver.Hasher, error) {
		return New(Params{
			N:          spec.ScryptN,
			R:          spec.ScryptR,
			P:          spec.ScryptP,
			KeyLength:  spec.ScryptKeyLength,
			SaltLength: spec.ScryptSaltLength,
		})
	})
}

// Params tunes the scrypt work factor.
type Params struct {
	N          int
	R          int
	P          int
	KeyLength  int
	SaltLength int
}

// DefaultParams matches the RFC 7914 recommendation for interactive
// logins (~100ms on commodity hardware in 2024): N=32768, r=8, p=1.
var DefaultParams = Params{
	N:          1 << 15,
	R:          8,
	P:          1,
	KeyLength:  32,
	SaltLength: 16,
}

// New constructs a scrypt Hasher. Zero fields fall back to DefaultParams.
func New(p Params) (hdriver.Hasher, error) {
	if p.N == 0 {
		p.N = DefaultParams.N
	}
	if p.R == 0 {
		p.R = DefaultParams.R
	}
	if p.P == 0 {
		p.P = DefaultParams.P
	}
	if p.KeyLength == 0 {
		p.KeyLength = DefaultParams.KeyLength
	}
	if p.SaltLength == 0 {
		p.SaltLength = DefaultParams.SaltLength
	}
	if p.N < 2 || (p.N&(p.N-1)) != 0 {
		return nil, fmt.Errorf("%w: scrypt N must be a power of 2 ≥ 2", hdriver.ErrIncompatibleParams)
	}
	if p.R < 1 || p.P < 1 || p.KeyLength < 16 {
		return nil, fmt.Errorf("%w: scrypt r, p, key length must be positive (key length ≥ 16)", hdriver.ErrIncompatibleParams)
	}
	return &hasher{p: p}, nil
}

type hasher struct{ p Params }

func (h *hasher) Name() string { return hdriver.DriverScrypt }

func (h *hasher) Make(_ context.Context, plain string) (string, error) {
	salt := make([]byte, h.p.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("hashing/scrypt: salt: %w", err)
	}
	digest, err := scrypt.Key([]byte(plain), salt, h.p.N, h.p.R, h.p.P, h.p.KeyLength)
	if err != nil {
		return "", fmt.Errorf("hashing/scrypt: %w", err)
	}
	return encode(h.p, salt, digest), nil
}

func (h *hasher) Check(_ context.Context, plain, hash string) (bool, error) {
	p, salt, digest, err := decode(hash)
	if err != nil {
		return false, err
	}
	candidate, err := scrypt.Key([]byte(plain), salt, p.N, p.R, p.P, len(digest))
	if err != nil {
		return false, fmt.Errorf("hashing/scrypt: %w", err)
	}
	if subtle.ConstantTimeCompare(candidate, digest) == 1 {
		return true, nil
	}
	return false, nil
}

func (h *hasher) NeedsRehash(hash string) bool {
	p, _, _, err := decode(hash)
	if err != nil {
		return true
	}
	return p.N < h.p.N || p.R < h.p.R || p.P < h.p.P
}

func (h *hasher) Info(hash string) (hdriver.Info, error) {
	p, _, _, err := decode(hash)
	if err != nil {
		return hdriver.Info{}, err
	}
	return hdriver.Info{
		Algorithm: hdriver.DriverScrypt,
		Params: map[string]string{
			"n": strconv.Itoa(p.N),
			"r": strconv.Itoa(p.R),
			"p": strconv.Itoa(p.P),
		},
	}, nil
}

func encode(p Params, salt, digest []byte) string {
	return fmt.Sprintf(
		"$scrypt$ln=%d,r=%d,p=%d$%s$%s",
		log2(p.N), p.R, p.P,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(digest),
	)
}

func decode(hash string) (Params, []byte, []byte, error) {
	if !strings.HasPrefix(hash, "$scrypt$") {
		return Params{}, nil, nil, hdriver.ErrUnknownFormat
	}
	parts := strings.Split(hash, "$")
	if len(parts) != 5 {
		return Params{}, nil, nil, hdriver.ErrInvalidHash
	}
	var ln int
	p := Params{}
	if _, err := fmt.Sscanf(parts[2], "ln=%d,r=%d,p=%d", &ln, &p.R, &p.P); err != nil {
		return Params{}, nil, nil, fmt.Errorf("%w: bad params: %w", hdriver.ErrInvalidHash, err)
	}
	if ln <= 0 || ln > 31 {
		return Params{}, nil, nil, fmt.Errorf("%w: ln out of range", hdriver.ErrInvalidHash)
	}
	p.N = 1 << uint(ln)
	salt, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return Params{}, nil, nil, fmt.Errorf("%w: bad salt: %w", hdriver.ErrInvalidHash, err)
	}
	digest, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return Params{}, nil, nil, fmt.Errorf("%w: bad digest: %w", hdriver.ErrInvalidHash, err)
	}
	p.SaltLength = len(salt)
	p.KeyLength = len(digest)
	return p, salt, digest, nil
}

func log2(n int) int {
	out := 0
	for n > 1 {
		n >>= 1
		out++
	}
	return out
}
