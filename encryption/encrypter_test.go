package encryption

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"
	"sync"
	"testing"

	edriver "github.com/godx-jp/godx-platform-framework/encryption/driver"
)

func newKey(t *testing.T, n int) []byte {
	t.Helper()
	k := make([]byte, n)
	if _, err := rand.Read(k); err != nil {
		t.Fatal(err)
	}
	return k
}

func newEnc(t *testing.T) *Encrypter {
	cipher, err := edriver.New(context.Background(), edriver.Spec{Name: edriver.DriverAESGCM})
	if err != nil {
		t.Fatal(err)
	}
	enc := NewEncrypter(cipher)
	if err := enc.AddKey("k1", newKey(t, 32)); err != nil {
		t.Fatal(err)
	}
	return enc
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	enc := newEnc(t)
	ctx := context.Background()
	tok, err := enc.EncryptString(ctx, "secret value")
	if err != nil {
		t.Fatalf("EncryptString: %v", err)
	}
	if !strings.HasPrefix(tok, "v1:k1:") {
		t.Fatalf("token prefix: %q", tok)
	}
	out, err := enc.DecryptString(ctx, tok)
	if err != nil {
		t.Fatalf("DecryptString: %v", err)
	}
	if out != "secret value" {
		t.Fatalf("round trip mismatch: %q", out)
	}
}

func TestKeyRotationOldTokensStillDecrypt(t *testing.T) {
	enc := newEnc(t)
	ctx := context.Background()
	oldTok, _ := enc.EncryptString(ctx, "old data")
	if err := enc.AddKey("k2", newKey(t, 32)); err != nil {
		t.Fatal(err)
	}
	if err := enc.SetPrimary("k2"); err != nil {
		t.Fatal(err)
	}
	newTok, _ := enc.EncryptString(ctx, "new data")
	if !strings.HasPrefix(newTok, "v1:k2:") {
		t.Fatalf("post-rotation token should use k2: %q", newTok)
	}
	if got, _ := enc.DecryptString(ctx, oldTok); got != "old data" {
		t.Fatalf("old token did not survive rotation: %q", got)
	}
	if got, _ := enc.DecryptString(ctx, newTok); got != "new data" {
		t.Fatalf("new token decrypt: %q", got)
	}
	if id, _ := KeyIDOf(oldTok); id != "k1" {
		t.Fatalf("KeyIDOf old: %q", id)
	}
}

func TestDecryptInvalidTokens(t *testing.T) {
	enc := newEnc(t)
	ctx := context.Background()
	cases := []struct {
		name  string
		token string
	}{
		{"missing colons", "abc"},
		{"wrong version", "v0:k1:abc"},
		{"empty key id", "v1::abc"},
		{"bad base64", "v1:k1:!!!"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			_, err := enc.DecryptString(ctx, c.token)
			if !errors.Is(err, edriver.ErrInvalidToken) {
				t.Fatalf("want ErrInvalidToken, got %v", err)
			}
		})
	}
}

func TestDecryptUnknownKeyID(t *testing.T) {
	enc := newEnc(t)
	ctx := context.Background()
	tok, _ := enc.EncryptString(ctx, "x")
	// Forge a token under a key id that's not registered.
	parts := strings.SplitN(tok, ":", 3)
	forged := "v1:other:" + parts[2]
	_, err := enc.DecryptString(ctx, forged)
	if !errors.Is(err, edriver.ErrUnknownKey) {
		t.Fatalf("want ErrUnknownKey, got %v", err)
	}
}

func TestEncryptWithNoPrimaryFails(t *testing.T) {
	cipher, _ := edriver.New(context.Background(), edriver.Spec{Name: edriver.DriverAESGCM})
	enc := NewEncrypter(cipher)
	_, err := enc.EncryptString(context.Background(), "x")
	if err == nil {
		t.Fatalf("expected error when no primary key")
	}
}

func TestAddKeyRejectsBadInputs(t *testing.T) {
	enc := newEnc(t)
	if err := enc.AddKey("", newKey(t, 32)); err == nil {
		t.Fatalf("empty id should error")
	}
	if err := enc.AddKey("k2", newKey(t, 10)); !errors.Is(err, edriver.ErrInvalidKeySize) {
		t.Fatalf("wrong-size key should error with ErrInvalidKeySize, got %v", err)
	}
	if err := enc.AddKey("k1", newKey(t, 32)); err == nil {
		t.Fatalf("duplicate id should error")
	}
}

func TestSetPrimaryUnknownRejected(t *testing.T) {
	enc := newEnc(t)
	if err := enc.SetPrimary("missing"); err == nil {
		t.Fatalf("SetPrimary unknown should error")
	}
}

func TestPrimaryAndKeyIDsReflectState(t *testing.T) {
	enc := newEnc(t)
	_ = enc.AddKey("k2", newKey(t, 32))
	if enc.PrimaryKeyID() != "k1" {
		t.Fatalf("primary should still be k1")
	}
	_ = enc.SetPrimary("k2")
	if enc.PrimaryKeyID() != "k2" {
		t.Fatalf("primary should be k2 after SetPrimary")
	}
	ids := enc.KeyIDs()
	if len(ids) != 2 {
		t.Fatalf("KeyIDs len: %d", len(ids))
	}
}

func TestEncryptConcurrentSafe(t *testing.T) {
	enc := newEnc(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			tok, err := enc.EncryptString(ctx, "x")
			if err != nil {
				t.Errorf("Encrypt: %v", err)
				return
			}
			if _, err := enc.DecryptString(ctx, tok); err != nil {
				t.Errorf("Decrypt: %v", err)
			}
		}()
		go func() {
			defer wg.Done()
			_ = enc.PrimaryKeyID()
			_ = enc.KeyIDs()
		}()
	}
	wg.Wait()
}

func TestParseKeyFormats(t *testing.T) {
	raw := newKey(t, 32)
	b64 := "base64:" + base64.StdEncoding.EncodeToString(raw)
	got, err := ParseKey(b64)
	if err != nil || string(got) != string(raw) {
		t.Fatalf("base64 parse: %v", err)
	}
	got, err = ParseKey("hex:00ff")
	if err != nil || len(got) != 2 || got[0] != 0 || got[1] != 0xff {
		t.Fatalf("hex parse: %v %v", got, err)
	}
	if _, err := ParseKey("hex:zz"); err == nil {
		t.Fatalf("hex parse invalid should error")
	}
	if _, err := ParseKey("base64:!!!"); err == nil {
		t.Fatalf("base64 parse invalid should error")
	}
	// raw fallback
	raw2, _ := ParseKey("literal-key-string")
	if string(raw2) != "literal-key-string" {
		t.Fatalf("raw fallback failed")
	}
}

func TestKeyIDOfInvalidToken(t *testing.T) {
	if _, err := KeyIDOf("not a token"); !errors.Is(err, edriver.ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}
