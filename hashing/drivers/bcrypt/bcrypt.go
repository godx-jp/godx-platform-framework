// Package bcrypt provides the bcrypt Hasher driver — Laravel's
// default Hash driver. Auto-registers itself when the parent
// hashing package is imported.
package bcrypt

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/bcrypt"

	hdriver "github.com/godx-jp/godx-platform-framework/hashing/driver"
)

func init() {
	hdriver.Register(hdriver.DriverBcrypt, func(_ context.Context, spec hdriver.Spec) (hdriver.Hasher, error) {
		return New(spec.BcryptCost)
	})
}

// New constructs a bcrypt Hasher with the given cost. cost <= 0
// falls back to the package default (12, two above bcrypt's library
// default to keep up with modern CPUs).
func New(cost int) (hdriver.Hasher, error) {
	if cost <= 0 {
		cost = 12
	}
	if cost < bcrypt.MinCost || cost > bcrypt.MaxCost {
		return nil, fmt.Errorf("%w: bcrypt cost %d outside [%d..%d]", hdriver.ErrIncompatibleParams, cost, bcrypt.MinCost, bcrypt.MaxCost)
	}
	return &hasher{cost: cost}, nil
}

type hasher struct{ cost int }

func (h *hasher) Name() string { return hdriver.DriverBcrypt }

// MaxPasswordBytes is the bcrypt input ceiling. The library will
// reject longer plaintext on Make/Check; we surface a clear error.
const MaxPasswordBytes = 72

func (h *hasher) Make(_ context.Context, plain string) (string, error) {
	if len(plain) > MaxPasswordBytes {
		return "", hdriver.ErrPasswordTooLong
	}
	b, err := bcrypt.GenerateFromPassword([]byte(plain), h.cost)
	if err != nil {
		return "", fmt.Errorf("hashing/bcrypt: %w", err)
	}
	return string(b), nil
}

func (h *hasher) Check(_ context.Context, plain, hash string) (bool, error) {
	if len(plain) > MaxPasswordBytes {
		return false, hdriver.ErrPasswordTooLong
	}
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain))
	if err == nil {
		return true, nil
	}
	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		return false, nil
	}
	if errors.Is(err, bcrypt.ErrHashTooShort) || strings.Contains(err.Error(), "hashedSecret too short") {
		return false, hdriver.ErrInvalidHash
	}
	return false, fmt.Errorf("hashing/bcrypt: %w", err)
}

func (h *hasher) NeedsRehash(hash string) bool {
	cost, err := bcrypt.Cost([]byte(hash))
	if err != nil {
		return true
	}
	return cost < h.cost
}

func (h *hasher) Info(hash string) (hdriver.Info, error) {
	if !strings.HasPrefix(hash, "$2") {
		return hdriver.Info{}, hdriver.ErrUnknownFormat
	}
	cost, err := bcrypt.Cost([]byte(hash))
	if err != nil {
		return hdriver.Info{}, hdriver.ErrInvalidHash
	}
	return hdriver.Info{
		Algorithm: hdriver.DriverBcrypt,
		Params:    map[string]string{"cost": strconv.Itoa(cost)},
	}, nil
}
