package hashing

import (
	"context"
	"errors"
	"strings"
	"testing"

	hdriver "github.com/godx-jp/godx-platform-framework/hashing/driver"
)

func TestBcryptNeedsRehashOnLowerCost(t *testing.T) {
	ctx := context.Background()
	low, _ := hdriver.New(ctx, hdriver.Spec{Name: hdriver.DriverBcrypt, BcryptCost: 4})
	hi, _ := hdriver.New(ctx, hdriver.Spec{Name: hdriver.DriverBcrypt, BcryptCost: 10})

	enc, err := low.Make(ctx, "x")
	if err != nil {
		t.Fatal(err)
	}
	if !hi.NeedsRehash(enc) {
		t.Fatalf("higher-cost hasher should mark lower-cost hash as rehash-needed")
	}
}

func TestBcryptPasswordTooLong(t *testing.T) {
	ctx := context.Background()
	h, _ := hdriver.New(ctx, hdriver.Spec{Name: hdriver.DriverBcrypt, BcryptCost: 4})
	tooLong := strings.Repeat("a", 80)
	_, err := h.Make(ctx, tooLong)
	if !errors.Is(err, hdriver.ErrPasswordTooLong) {
		t.Fatalf("Make: expected ErrPasswordTooLong, got %v", err)
	}
	if _, err := h.Check(ctx, tooLong, "$2a$04$abc"); !errors.Is(err, hdriver.ErrPasswordTooLong) {
		t.Fatalf("Check: expected ErrPasswordTooLong, got %v", err)
	}
}

func TestBcryptInvalidHash(t *testing.T) {
	ctx := context.Background()
	h, _ := hdriver.New(ctx, hdriver.Spec{Name: hdriver.DriverBcrypt, BcryptCost: 4})
	if _, err := h.Check(ctx, "x", "not-a-bcrypt-hash"); err == nil {
		t.Fatalf("expected error for invalid hash")
	}
	if _, err := h.Info("not-a-bcrypt-hash"); !errors.Is(err, hdriver.ErrUnknownFormat) {
		t.Fatalf("expected ErrUnknownFormat for non-$2 hash, got %v", err)
	}
}

func TestBcryptCostRangeRejected(t *testing.T) {
	_, err := hdriver.New(context.Background(), hdriver.Spec{Name: hdriver.DriverBcrypt, BcryptCost: 99})
	if !errors.Is(err, hdriver.ErrIncompatibleParams) {
		t.Fatalf("expected ErrIncompatibleParams, got %v", err)
	}
}

func TestArgon2idNeedsRehashOnLowerCost(t *testing.T) {
	ctx := context.Background()
	low, _ := hdriver.New(ctx, hdriver.Spec{Name: hdriver.DriverArgon2id, Argon2Time: 1, Argon2Memory: 8 * 1024, Argon2Threads: 1})
	hi, _ := hdriver.New(ctx, hdriver.Spec{Name: hdriver.DriverArgon2id, Argon2Time: 3, Argon2Memory: 64 * 1024, Argon2Threads: 2})
	enc, _ := low.Make(ctx, "x")
	if !hi.NeedsRehash(enc) {
		t.Fatalf("higher-cost argon2id should mark lower-cost hash as needing rehash")
	}
}

func TestArgon2idInvalidHashes(t *testing.T) {
	ctx := context.Background()
	h, _ := hdriver.New(ctx, hdriver.Spec{Name: hdriver.DriverArgon2id})

	cases := []string{
		"$argon2id$v=19$m=65536,t=3,p=2$bad-base64!@#$YmJi",            // bad salt base64
		"$argon2id$bad",                                                // too few segments
		"$argon2i$v=19$m=65536,t=3,p=2$YWFh$YmJi",                     // wrong variant (argon2i not argon2id)
		"$argon2id$v=18$m=65536,t=3,p=2$YWFh$YmJi",                    // unsupported version
	}
	for _, bad := range cases {
		if _, err := h.Check(ctx, "x", bad); err == nil {
			t.Errorf("Check(%q) should error", bad)
		}
	}
}

func TestArgon2idMemoryParamRejected(t *testing.T) {
	// memory < 8*threads
	_, err := hdriver.New(context.Background(), hdriver.Spec{Name: hdriver.DriverArgon2id, Argon2Memory: 1, Argon2Threads: 8})
	if !errors.Is(err, hdriver.ErrIncompatibleParams) {
		t.Fatalf("expected ErrIncompatibleParams, got %v", err)
	}
}

func TestScryptNeedsRehashOnLowerCost(t *testing.T) {
	ctx := context.Background()
	low, _ := hdriver.New(ctx, hdriver.Spec{Name: hdriver.DriverScrypt, ScryptN: 1 << 10, ScryptR: 8, ScryptP: 1})
	hi, _ := hdriver.New(ctx, hdriver.Spec{Name: hdriver.DriverScrypt, ScryptN: 1 << 12, ScryptR: 8, ScryptP: 1})
	enc, _ := low.Make(ctx, "x")
	if !hi.NeedsRehash(enc) {
		t.Fatalf("higher-N scrypt should mark lower-N hash as needing rehash")
	}
}

func TestScryptNotPowerOfTwoRejected(t *testing.T) {
	_, err := hdriver.New(context.Background(), hdriver.Spec{Name: hdriver.DriverScrypt, ScryptN: 1000})
	if !errors.Is(err, hdriver.ErrIncompatibleParams) {
		t.Fatalf("expected ErrIncompatibleParams, got %v", err)
	}
}

func TestScryptInvalidHashes(t *testing.T) {
	ctx := context.Background()
	h, _ := hdriver.New(ctx, hdriver.Spec{Name: hdriver.DriverScrypt})
	if _, err := h.Check(ctx, "x", "$scrypt$bad"); err == nil {
		t.Fatalf("expected error on bad scrypt hash")
	}
	if _, err := h.Info("$scrypt$ln=15,r=8,p=1$BADSALT!!$BADDIGEST!!"); err == nil {
		t.Fatalf("expected error on bad base64")
	}
}

func TestCrossDriverInfoUnknown(t *testing.T) {
	ctx := context.Background()
	bcr, _ := hdriver.New(ctx, hdriver.Spec{Name: hdriver.DriverBcrypt, BcryptCost: 4})
	ar, _ := hdriver.New(ctx, hdriver.Spec{Name: hdriver.DriverArgon2id, Argon2Time: 1, Argon2Memory: 8 * 1024, Argon2Threads: 1})

	bcEnc, _ := bcr.Make(ctx, "x")
	arEnc, _ := ar.Make(ctx, "x")
	if _, err := ar.Info(bcEnc); !errors.Is(err, hdriver.ErrUnknownFormat) {
		t.Fatalf("argon2id should not recognise bcrypt hash, got %v", err)
	}
	if _, err := bcr.Info(arEnc); !errors.Is(err, hdriver.ErrUnknownFormat) {
		t.Fatalf("bcrypt should not recognise argon2id hash, got %v", err)
	}
}

func TestMustDefaultReturnsBcrypt(t *testing.T) {
	h := MustDefault()
	if h.Name() != hdriver.DriverBcrypt {
		t.Fatalf("MustDefault: %q", h.Name())
	}
	enc, err := h.Make(context.Background(), "x")
	if err != nil {
		t.Fatalf("Make: %v", err)
	}
	if !strings.HasPrefix(enc, "$2") {
		t.Fatalf("MustDefault did not produce bcrypt hash: %q", enc)
	}
}
