package hashing

import (
	"context"
	"errors"
	"strings"
	"testing"

	hdriver "github.com/godx-jp/godx-platform-framework/hashing/driver"
)

// TestConformanceHashers runs the driver-agnostic behaviour suite
// across every registered Hasher.
func TestConformanceHashers(t *testing.T) {
	cases := []struct {
		name string
		spec hdriver.Spec
	}{
		{name: "bcrypt", spec: hdriver.Spec{Name: hdriver.DriverBcrypt, BcryptCost: 4}}, // min cost for fast tests
		{name: "argon2id", spec: hdriver.Spec{Name: hdriver.DriverArgon2id, Argon2Time: 1, Argon2Memory: 8 * 1024, Argon2Threads: 1, Argon2KeyLength: 32, Argon2SaltLength: 16}},
		{name: "scrypt", spec: hdriver.Spec{Name: hdriver.DriverScrypt, ScryptN: 1 << 10, ScryptR: 8, ScryptP: 1}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			h, err := hdriver.New(ctx, tc.spec)
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			if h.Name() != tc.spec.Name {
				t.Fatalf("Name: %q", h.Name())
			}

			plain := "hunter2-very-secret-password"
			enc, err := h.Make(ctx, plain)
			if err != nil {
				t.Fatalf("Make: %v", err)
			}
			if enc == "" || enc == plain {
				t.Fatalf("encoded hash unexpected: %q", enc)
			}

			ok, err := h.Check(ctx, plain, enc)
			if err != nil || !ok {
				t.Fatalf("Check: ok=%v err=%v", ok, err)
			}

			ok, err = h.Check(ctx, "wrong", enc)
			if err != nil || ok {
				t.Fatalf("Check wrong: ok=%v err=%v", ok, err)
			}

			info, err := h.Info(enc)
			if err != nil {
				t.Fatalf("Info: %v", err)
			}
			if info.Algorithm != tc.spec.Name {
				t.Fatalf("Info Algorithm: %q", info.Algorithm)
			}
			if len(info.Params) == 0 {
				t.Fatalf("Info Params empty")
			}

			if h.NeedsRehash(enc) {
				t.Fatalf("NeedsRehash should be false right after Make")
			}

			if _, err := h.Info("$nope$invalid"); !errors.Is(err, hdriver.ErrUnknownFormat) && !errors.Is(err, hdriver.ErrInvalidHash) {
				t.Fatalf("Info on garbage: %v", err)
			}

			// Each hash differs by random salt — two Makes of the same plaintext
			// must produce different ciphertexts.
			enc2, _ := h.Make(ctx, plain)
			if enc == enc2 {
				t.Fatalf("hashes are deterministic (salt missing?)")
			}
		})
	}
}

func TestRegistryHelpfulMissingDriverError(t *testing.T) {
	_, err := hdriver.New(context.Background(), hdriver.Spec{Name: "definitely-missing"})
	if err == nil || !strings.Contains(err.Error(), "drivers/definitely-missing") {
		t.Fatalf("expected drivers/<name> import hint, got %v", err)
	}
}
