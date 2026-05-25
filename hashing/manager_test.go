package hashing

import (
	"context"
	"strings"
	"testing"

	hdriver "github.com/godx-jp/godx-platform-framework/hashing/driver"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	ctx := context.Background()
	bcr, err := hdriver.New(ctx, hdriver.Spec{Name: hdriver.DriverBcrypt, BcryptCost: 4})
	if err != nil {
		t.Fatal(err)
	}
	ar, err := hdriver.New(ctx, hdriver.Spec{Name: hdriver.DriverArgon2id, Argon2Time: 1, Argon2Memory: 8 * 1024, Argon2Threads: 1})
	if err != nil {
		t.Fatal(err)
	}
	mgr := NewManager()
	if err := mgr.AddHasher("legacy", bcr); err != nil {
		t.Fatal(err)
	}
	if err := mgr.AddHasher("new", ar); err != nil {
		t.Fatal(err)
	}
	if err := mgr.SetDefault("new"); err != nil {
		t.Fatal(err)
	}
	return mgr
}

func TestManagerDefaultAndNamed(t *testing.T) {
	mgr := newTestManager(t)
	def := mgr.Default()
	if def == nil || def.Name() != hdriver.DriverArgon2id {
		t.Fatalf("Default wrong: %v", def)
	}
	legacy, err := mgr.Hasher("legacy")
	if err != nil || legacy.Name() != hdriver.DriverBcrypt {
		t.Fatalf("legacy lookup wrong: %v err=%v", legacy, err)
	}
	if _, err := mgr.Hasher("missing"); err == nil {
		t.Fatalf("missing should error")
	}
	names := mgr.Hashers()
	if len(names) != 2 || names[0] != "legacy" || names[1] != "new" {
		t.Fatalf("Hashers names sort wrong: %v", names)
	}
}

func TestManagerDuplicateRejected(t *testing.T) {
	mgr := newTestManager(t)
	bcr, _ := hdriver.New(context.Background(), hdriver.Spec{Name: hdriver.DriverBcrypt, BcryptCost: 4})
	if err := mgr.AddHasher("legacy", bcr); err == nil {
		t.Fatalf("duplicate should error")
	}
}

func TestManagerNilHasher(t *testing.T) {
	mgr := NewManager()
	if err := mgr.AddHasher("x", nil); err == nil {
		t.Fatalf("nil should error")
	}
	if err := mgr.AddHasher("", nil); err == nil {
		t.Fatalf("empty name should error")
	}
}

func TestManagerSetDefaultUnknown(t *testing.T) {
	mgr := NewManager()
	if err := mgr.SetDefault("nope"); err == nil {
		t.Fatalf("SetDefault unknown should error")
	}
}

func TestManagerCheckAnyMixedHashes(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()

	bcr, _ := mgr.Hasher("legacy")
	bcEnc, err := bcr.Make(ctx, "secret")
	if err != nil {
		t.Fatal(err)
	}

	ar, _ := mgr.Hasher("new")
	arEnc, err := ar.Make(ctx, "secret")
	if err != nil {
		t.Fatal(err)
	}

	ok, name, err := mgr.CheckAny(ctx, "secret", bcEnc)
	if err != nil || !ok || name != "legacy" {
		t.Fatalf("CheckAny bcrypt: ok=%v name=%q err=%v", ok, name, err)
	}
	ok, name, err = mgr.CheckAny(ctx, "secret", arEnc)
	if err != nil || !ok || name != "new" {
		t.Fatalf("CheckAny argon2: ok=%v name=%q err=%v", ok, name, err)
	}

	if _, _, err := mgr.CheckAny(ctx, "secret", "$nope$bad"); err == nil || !strings.Contains(err.Error(), "no registered hasher") {
		t.Fatalf("unknown encoding should error, got %v", err)
	}
}

func TestManagerShutdownNoop(t *testing.T) {
	mgr := newTestManager(t)
	if err := mgr.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
}
