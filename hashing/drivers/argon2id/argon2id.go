// Package argon2id provides the Argon2id Hasher driver. Encoded
// output uses the PHC string format ($argon2id$v=19$m=...,t=...,p=...$salt$hash)
// which Laravel and most argon2 implementations interoperate with.
package argon2id

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"

	hdriver "github.com/godx-jp/godx-platform-framework/hashing/driver"
)

func init() {
	hdriver.Register(hdriver.DriverArgon2id, func(_ context.Context, spec hdriver.Spec) (hdriver.Hasher, error) {
		return New(Params{
			Time:       spec.Argon2Time,
			Memory:     spec.Argon2Memory,
			Threads:    spec.Argon2Threads,
			KeyLength:  spec.Argon2KeyLength,
			SaltLength: spec.Argon2SaltLength,
		})
	})
}

// Params tunes the Argon2id work factor.
type Params struct {
	Time       uint32
	Memory     uint32
	Threads    uint8
	KeyLength  uint32
	SaltLength uint32
}

// DefaultParams matches OWASP's 2024 recommendation: 64 MiB memory,
// 3 iterations, 2 lanes — strong yet sub-100ms on commodity hardware.
var DefaultParams = Params{
	Time:       3,
	Memory:     64 * 1024,
	Threads:    2,
	KeyLength:  32,
	SaltLength: 16,
}

// New constructs an Argon2id Hasher. Zero fields fall back to DefaultParams.
func New(p Params) (hdriver.Hasher, error) {
	if p.Time == 0 {
		p.Time = DefaultParams.Time
	}
	if p.Memory == 0 {
		p.Memory = DefaultParams.Memory
	}
	if p.Threads == 0 {
		p.Threads = DefaultParams.Threads
	}
	if p.KeyLength == 0 {
		p.KeyLength = DefaultParams.KeyLength
	}
	if p.SaltLength == 0 {
		p.SaltLength = DefaultParams.SaltLength
	}
	if p.Memory < 8*uint32(p.Threads) {
		return nil, fmt.Errorf("%w: argon2id memory must be at least 8*threads", hdriver.ErrIncompatibleParams)
	}
	return &hasher{p: p}, nil
}

type hasher struct{ p Params }

func (h *hasher) Name() string { return hdriver.DriverArgon2id }

func (h *hasher) Make(_ context.Context, plain string) (string, error) {
	salt := make([]byte, h.p.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("hashing/argon2id: salt: %w", err)
	}
	digest := argon2.IDKey([]byte(plain), salt, h.p.Time, h.p.Memory, h.p.Threads, h.p.KeyLength)
	return encode(h.p, salt, digest), nil
}

func (h *hasher) Check(_ context.Context, plain, hash string) (bool, error) {
	p, salt, digest, err := decode(hash)
	if err != nil {
		return false, err
	}
	candidate := argon2.IDKey([]byte(plain), salt, p.Time, p.Memory, p.Threads, uint32(len(digest)))
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
	return p.Time < h.p.Time ||
		p.Memory < h.p.Memory ||
		p.Threads < h.p.Threads
}

func (h *hasher) Info(hash string) (hdriver.Info, error) {
	p, _, _, err := decode(hash)
	if err != nil {
		return hdriver.Info{}, err
	}
	return hdriver.Info{
		Algorithm: hdriver.DriverArgon2id,
		Params: map[string]string{
			"time":    strconv.FormatUint(uint64(p.Time), 10),
			"memory":  strconv.FormatUint(uint64(p.Memory), 10),
			"threads": strconv.FormatUint(uint64(p.Threads), 10),
		},
	}, nil
}

func encode(p Params, salt, digest []byte) string {
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		p.Memory, p.Time, p.Threads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(digest),
	)
}

func decode(hash string) (Params, []byte, []byte, error) {
	if !strings.HasPrefix(hash, "$argon2id$") {
		return Params{}, nil, nil, hdriver.ErrUnknownFormat
	}
	parts := strings.Split(hash, "$")
	// expected: ["", "argon2id", "v=...", "m=...,t=...,p=...", "<salt>", "<digest>"]
	if len(parts) != 6 {
		return Params{}, nil, nil, hdriver.ErrInvalidHash
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return Params{}, nil, nil, fmt.Errorf("%w: bad version segment", hdriver.ErrInvalidHash)
	}
	if version != argon2.Version {
		return Params{}, nil, nil, fmt.Errorf("%w: unsupported argon2 version %d", hdriver.ErrInvalidHash, version)
	}
	p := Params{}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.Memory, &p.Time, &p.Threads); err != nil {
		return Params{}, nil, nil, fmt.Errorf("%w: bad params segment: %w", hdriver.ErrInvalidHash, err)
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return Params{}, nil, nil, fmt.Errorf("%w: bad salt: %w", hdriver.ErrInvalidHash, err)
	}
	digest, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return Params{}, nil, nil, fmt.Errorf("%w: bad digest: %w", hdriver.ErrInvalidHash, err)
	}
	if len(digest) == 0 {
		return Params{}, nil, nil, errors.New("hashing/argon2id: empty digest")
	}
	p.SaltLength = uint32(len(salt))
	p.KeyLength = uint32(len(digest))
	return p, salt, digest, nil
}
